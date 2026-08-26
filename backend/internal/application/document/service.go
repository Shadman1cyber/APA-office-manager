package documentsvc

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/apa/backend/internal/ai"
	"github.com/apa/backend/internal/application"
	"github.com/apa/backend/internal/domain"
	domainedocument "github.com/apa/backend/internal/domain/document"
	"github.com/apa/backend/internal/domain/task"
	"github.com/apa/backend/internal/domain/user"
)

const GenerateJobType = "generate_document"

type Deps struct {
	Documents application.DocumentRepository
	Tasks     application.TaskRepository
	Bus       *application.Bus
	Jobs      application.JobEnqueuer
	Agent     ai.DocumentationAgent
	Guard     ai.DocGuardAgent
	Fallback  ai.DocumentationAgent
	Log       *slog.Logger
}

type Service struct {
	deps Deps
}

func NewService(deps Deps) *Service {
	return &Service{deps: deps}
}

type CreateResult struct {
	Document *domainedocument.Document `json:"document"`
	Message  string                    `json:"message"`
}

// CreateAndGenerate: employee submits what he did; a document skeleton is stored and the
// AI agent fills it immediately (resilient: LLM -> deterministic fallback).
func (s *Service) CreateAndGenerate(ctx context.Context, actor *user.User, taskID *uuid.UUID, content string) (*CreateResult, error) {
	content = trimSpace(content)
	if err := domainedocument.ValidateSource(content); err != nil {
		return nil, err
	}

	title := "در حال تولید سند"

	if verdict, gerr := s.deps.Guard.CheckNotes(ctx, actor.Name, content); gerr == nil && !verdict.Safe {
		return nil, domain.Invalid("content", verdict.Reason)
	} else if gerr != nil {
		s.deps.Log.ErrorContext(ctx, "notes guard unavailable", slog.Any("error", gerr))
		return nil, fmt.Errorf("بررسی امنیتی یادداشت ممکن نشد؛ دوباره تلاش کنید")
	}

	doc := &domainedocument.Document{
		OrgID:       actor.OrgID,
		TaskID:      taskID,
		AuthorID:    actor.ID,
		AuthorName:  actor.Name,
		SourceNotes: content,
		Status:      domainedocument.StatusGenerating,
		Title:       title,
	}

	if taskID != nil {
		t, err := s.deps.Tasks.Get(ctx, actor.OrgID, *taskID)
		if err != nil {
			return nil, err
		}
		if !actor.Role.CanApprove() && (t.AssignedTo == nil || *t.AssignedTo != actor.ID) {
			return nil, fmt.Errorf("%w: فقط وظایف تخصیص‌یافته به خودتان قابل مستندسازی است", domain.ErrForbidden)
		}
		doc.WorkflowID = &t.WorkflowID
		title = "سند وظیفه: " + t.Title
		doc.Title = title
	}

	if err := s.deps.Documents.Create(ctx, doc); err != nil {
		return nil, fmt.Errorf("ثبت سند: %w", err)
	}

	if err := s.generate(ctx, doc); err != nil {
		domainedocument.MarkFailed(doc, err.Error())
		if uerr := s.deps.Documents.UpdateResult(ctx, doc.ID, doc.Title, doc.Body, domainedocument.StatusFailed); uerr != nil {
			s.deps.Log.ErrorContext(ctx, "mark document failed", slog.Any("error", uerr))
		}
		return nil, fmt.Errorf("تولید سند ناموفق بود؛ دوباره تلاش کنید")
	}

	return &CreateResult{
		Document: doc,
		Message:  "سند با موفقیت تولید شد.",
	}, nil
}

// EnqueueForTask: called when an employee completes a task; the document is generated in background.
func (s *Service) EnqueueForTask(ctx context.Context, actor *user.User, t *task.Task, notes string) error {
	doc := &domainedocument.Document{
		OrgID:       actor.OrgID,
		TaskID:      &t.ID,
		WorkflowID:  &t.WorkflowID,
		AuthorID:    actor.ID,
		AuthorName:  actor.Name,
		Title:       "در حال تولید سند «" + t.Title + "»",
		SourceNotes: notes,
		Status:      domainedocument.StatusGenerating,
	}
	if err := s.deps.Documents.Create(ctx, doc); err != nil {
		return fmt.Errorf("create document skeleton: %w", err)
	}
	payload, _ := json.Marshal(map[string]string{"document_id": doc.ID.String(), "org_id": actor.OrgID.String()})
	if err := s.deps.Jobs.Enqueue(ctx, GenerateJobType, payload, time.Now().UTC().Add(500*time.Millisecond)); err != nil {
		return fmt.Errorf("enqueue document job: %w", err)
	}
	return nil
}

// ProcessGeneration is executed by the background worker.
func (s *Service) ProcessGeneration(ctx context.Context, orgID, documentID uuid.UUID) error {
	doc, err := s.deps.Documents.Get(ctx, orgID, documentID)
	if err != nil {
		return fmt.Errorf("load document: %w", err)
	}

	input := ai.DocumentInput{
		TaskTitle:  stripPrefix(doc.Title),
		AuthorName: doc.AuthorName,
		RawNotes:   doc.SourceNotes,
	}
	if doc.TaskID != nil {
		if t, terr := s.deps.Tasks.Get(ctx, orgID, *doc.TaskID); terr == nil {
			input.TaskTitle = t.Title
			input.Topic = t.Topic
			input.ExpectedOutput = t.ExpectedOutput
		}
	}

	body, gerr := s.generateValidatedBody(ctx, input)
	if gerr != nil {
		domainedocument.MarkFailed(doc, gerr.Error())
		_ = s.deps.Documents.UpdateResult(ctx, doc.ID, doc.Title, doc.Body, domainedocument.StatusFailed)
		return fmt.Errorf("documentation agent: %w", gerr)
	}

	title := input.TaskTitle
	domainedocument.MarkReady(doc, title, body)
	if err := s.deps.Documents.UpdateResult(ctx, doc.ID, doc.Title, doc.Body, domainedocument.StatusReady); err != nil {
		return fmt.Errorf("save document: %w", err)
	}

	s.deps.Bus.Publish(ctx, &application.Event{
		Type:       application.EventDocumentCreated,
		OrgID:      orgID,
		EntityType: "document",
		EntityID:   doc.ID.String(),
		ActorID:    &doc.AuthorID,
		Payload: map[string]any{
			"title":       doc.Title,
			"author":      doc.AuthorName,
			"task_id":     taskIdString(doc.TaskID),
			"workflow_id": workflowIdString(doc.WorkflowID),
		},
	})
	return nil
}

func (s *Service) List(ctx context.Context, actor *user.User, limit int) ([]*domainedocument.Document, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	if actor.Role.CanApprove() {
		return s.deps.Documents.ListByOrg(ctx, actor.OrgID, limit)
	}
	return s.deps.Documents.ListByAuthor(ctx, actor.OrgID, actor.ID, limit)
}

func (s *Service) Get(ctx context.Context, actor *user.User, id uuid.UUID) (*domainedocument.Document, error) {
	doc, err := s.deps.Documents.Get(ctx, actor.OrgID, id)
	if err != nil {
		return nil, err
	}
	if !actor.Role.CanApprove() && doc.AuthorID != actor.ID {
		return nil, fmt.Errorf("%w: این سند متعلق به شما نیست", domain.ErrForbidden)
	}
	return doc, nil
}

func (s *Service) generate(ctx context.Context, doc *domainedocument.Document) error {
	input := ai.DocumentInput{
		TaskTitle:  stripPrefix(doc.Title),
		Topic:      "",
		AuthorName: doc.AuthorName,
		RawNotes:   doc.SourceNotes,
	}
	if doc.TaskID != nil {
		if t, terr := s.deps.Tasks.Get(ctx, doc.OrgID, *doc.TaskID); terr == nil {
			input.TaskTitle = t.Title
			input.Topic = t.Topic
			input.ExpectedOutput = t.ExpectedOutput
		}
	}
	body, err := s.generateValidatedBody(ctx, input)
	if err != nil {
		return err
	}
	domainedocument.MarkReady(doc, firstNonEmptyStr(generatedTitleFromBody(body), input.TaskTitle), body)
	if err := s.deps.Documents.UpdateResult(ctx, doc.ID, doc.Title, doc.Body, domainedocument.StatusReady); err != nil {
		return err
	}
	s.deps.Bus.Publish(ctx, &application.Event{
		Type:       application.EventDocumentCreated,
		OrgID:      doc.OrgID,
		EntityType: "document",
		EntityID:   doc.ID.String(),
		ActorID:    &doc.AuthorID,
		Payload: map[string]any{
			"title":   doc.Title,
			"author":  doc.AuthorName,
			"task_id": taskIdString(doc.TaskID),
		},
	})
	return nil
}

func trimSpace(s string) string {
	out := []rune(s)
	start, end := 0, len(out)-1
	for start <= end && (out[start] == ' ' || out[start] == '\n' || out[start] == '\t' || out[start] == '\r') {
		start++
	}
	for end >= start && (out[end] == ' ' || out[end] == '\n' || out[end] == '\t' || out[end] == '\r') {
		end--
	}
	if start > end {
		return ""
	}
	return string(out[start : end+1])
}

func stripPrefix(title string) string {
	const prefix = "سند وظیفه: "
	if len(title) > len(prefix) && title[:len(prefix)] == prefix {
		return title[len(prefix):]
	}
	return title
}

func taskIdString(id *uuid.UUID) any {
	if id == nil {
		return nil
	}
	return id.String()
}

func workflowIdString(id *uuid.UUID) any {
	if id == nil {
		return nil
	}
	return id.String()
}

// generateValidatedBody runs the documentation agent and validates the result.
// If the LLM output is rejected by the coherence/grounding judge it retries once;
// on second failure it falls back to the deterministic template which is grounded
// by construction.
func (s *Service) generateValidatedBody(ctx context.Context, input ai.DocumentInput) (string, error) {
	var lastFeedback string
	for attempt := 0; attempt < 2; attempt++ {
		generated, err := s.deps.Agent.GenerateDocument(ctx, input)
		if err != nil {
			lastFeedback = err.Error()
			continue
		}

		verdict, gerr := s.deps.Guard.CheckDocument(ctx, input, generated.Body)
		if gerr != nil {
			s.deps.Log.ErrorContext(ctx, "document guard unavailable", slog.Any("error", gerr))
			return "", fmt.Errorf("داوری کیفیت سند ممکن نشد")
		}
		if !verdict.MakesSense || !verdict.Grounded {
			lastFeedback = verdict.Feedback
			s.deps.Log.WarnContext(ctx, "generated document rejected by guard; retrying",
				slog.String("feedback", verdict.Feedback), slog.Int("attempt", attempt+1))
			continue
		}
		return generated.Body, nil
	}

	s.deps.Log.WarnContext(ctx, "falling back to deterministic document template",
		slog.String("last_feedback", lastFeedback))
	fallbackDoc, ferr := s.deps.Fallback.GenerateDocument(ctx, input)
	if ferr != nil {
		return "", fmt.Errorf("%v (fallback also failed: %w)", lastFeedback, ferr)
	}
	return fallbackDoc.Body, nil
}

func generatedTitleFromBody(body string) string {
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			return strings.TrimPrefix(line, "# ")
		}
	}
	return ""
}

func firstNonEmptyStr(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}
