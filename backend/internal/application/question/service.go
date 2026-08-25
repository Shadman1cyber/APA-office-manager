package questionsvc

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/apa/backend/internal/ai"
	"github.com/apa/backend/internal/application"
	"github.com/apa/backend/internal/application/assignment"
	"github.com/apa/backend/internal/application/knowledge"
	"github.com/apa/backend/internal/application/task"
	"github.com/apa/backend/internal/domain"
	"github.com/apa/backend/internal/domain/knowledge"
	"github.com/apa/backend/internal/domain/question"
	domaintask "github.com/apa/backend/internal/domain/task"
	"github.com/apa/backend/internal/domain/user"
)

type Deps struct {
	Questions    application.QuestionRepository
	Tasks        application.TaskRepository
	Users        application.UserRepository
	Knowledge    *knowledgesvc.Service
	TasksService *tasksvc.Service
	Orchestrator *ai.Orchestrator
	Bus          *application.Bus
	Log          *slog.Logger
}

type Service struct {
	deps Deps
}

func NewService(deps Deps) *Service {
	return &Service{deps: deps}
}

type AnswerResult struct {
	Question *question.Question `json:"question"`
	Learned  string             `json:"learned,omitempty"`
	Task     *domaintask.Task   `json:"task,omitempty"`
}

func (s *Service) Get(ctx context.Context, orgID, id uuid.UUID) (*question.Question, error) {
	return s.deps.Questions.Get(ctx, orgID, id)
}

func (s *Service) List(ctx context.Context, orgID uuid.UUID, status string, workflowID *uuid.UUID) ([]*question.Question, error) {
	return s.deps.Questions.ListByOrg(ctx, orgID, status, workflowID)
}

func (s *Service) OpenForWorkflow(ctx context.Context, orgID, workflowID uuid.UUID) ([]*question.Question, error) {
	return s.deps.Questions.ListOpenRequired(ctx, workflowID)
}

func (s *Service) Answer(ctx context.Context, actor *user.User, id uuid.UUID, answerText string) (*AnswerResult, error) {
	answerText = strings.TrimSpace(answerText)
	if !actor.Role.CanApprove() {
		return nil, fmt.Errorf("%w: پاسخ‌دادن به سؤال‌های برنامه فقط برای مدیران مجاز است", domain.ErrForbidden)
	}
	q, err := s.deps.Questions.Get(ctx, actor.OrgID, id)
	if err != nil {
		return nil, err
	}
	if err := q.ApplyAnswer(answerText, actor.ID, time.Now().UTC()); err != nil {
		return nil, err
	}
	if err := s.deps.Questions.PersistAnswer(ctx, q); err != nil {
		return nil, err
	}
	s.publish(ctx, actor, application.EventQuestionAnswered, "question", q.ID, map[string]any{
		"workflow_id": q.WorkflowID.String(),
		"topic":       q.Topic,
	})

	result := &AnswerResult{Question: q}

	people, err := s.deps.Users.List(ctx, actor.OrgID)
	if err != nil {
		return nil, err
	}
	orgPeople := ai.NewOrgContext(people, nil).People

	learning, lerr := s.deps.Orchestrator.LearnFromAnswer(ctx, q.Answer, q.Topic, orgPeople)
	if lerr != nil {
		s.deps.Log.WarnContext(ctx, "learning agent failed", slog.Any("error", lerr))
	} else if learning.PersonID != nil && !answerMentionsPerson(learning, orgPeople, q.Answer) {
		s.deps.Log.WarnContext(ctx, "learning agent mapped to a person whose name is not in the answer; ignoring",
			slog.String("person_id", *learning.PersonID))
		learning.PersonID = nil
	}
	if lerr == nil && learning.PersonID != nil {
		personID, perr := uuid.Parse(*learning.PersonID)
		if perr != nil {
			s.deps.Log.WarnContext(ctx, "learning agent returned invalid person id", slog.String("person_id", *learning.PersonID))
		} else if fact := s.learnFact(ctx, actor.OrgID, q.Topic, personID, q.Answer); fact != nil {
			name := personDisplayName(orgPeople, personID, learning.PersonName)
			result.Learned = fmt.Sprintf("ثبت شد: %s مسئول «%s» است (اطمینان %.0f%%).", name, fact.Subject, fact.Confidence*100)
			s.publish(ctx, actor, application.EventKnowledgeLearned, "knowledge", fact.ID, map[string]any{
				"subject":    fact.Subject,
				"person":     name,
				"confidence": fact.Confidence,
				"source":     string(fact.Source),
			})
		}
	}

	if q.RelatedTaskID != nil && s.deps.TasksService != nil {
		t, terr := s.deps.Tasks.Get(ctx, actor.OrgID, *q.RelatedTaskID)
		if terr != nil {
			return result, nil
		}
		if t.Status != domaintask.StatusProposed {
			return result, nil
		}
		proposal, aerr := s.deps.Orchestrator.ReassignTask(ctx, actor.OrgID, t)
		if aerr != nil {
			s.deps.Log.WarnContext(ctx, "re-assignment agent failed", slog.Any("error", aerr))
			return result, nil
		}
		p, perr := assignmentsvc.FromAI(proposal)
		if perr != nil {
			s.deps.Log.WarnContext(ctx, "invalid re-assignment proposal", slog.Any("error", perr))
			return result, nil
		}
		if p.CandidateUserID == nil {
			return result, nil
		}
		if serr := s.deps.Tasks.SaveProposal(ctx, t.ID, p); serr != nil {
			s.deps.Log.WarnContext(ctx, "saving re-assignment proposal failed", slog.Any("error", serr))
			return result, nil
		}
		s.publish(ctx, actor, application.EventAssignmentProposed, "task", t.ID, map[string]any{
			"task_id":    t.ID.String(),
			"task_title": t.Title,
			"candidate":  p.CandidateName,
			"confidence": p.Confidence,
			"evidence":   p.Evidence,
		})
		if updated, uerr := s.deps.Tasks.Get(ctx, actor.OrgID, t.ID); uerr == nil {
			result.Task = updated
		}
	}

	return result, nil
}

func (s *Service) learnFact(ctx context.Context, orgID uuid.UUID, topic string, personID uuid.UUID, evidence string) *knowledge.Fact {
	if strings.TrimSpace(topic) == "" {
		return nil
	}
	fact, err := s.deps.Knowledge.LearnFromAnswer(ctx, orgID, topic, personID, truncate(evidence))
	if err != nil {
		s.deps.Log.WarnContext(ctx, "knowledge recording failed", slog.Any("error", err))
		return nil
	}
	return fact
}

func truncate(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 200 {
		return s[:200]
	}
	return s
}

func personDisplayName(people []ai.PersonInfo, id uuid.UUID, fallback string) string {
	for _, p := range people {
		if p.ID == id {
			return p.Name
		}
	}
	return fallback
}

func (s *Service) publish(ctx context.Context, actor *user.User, typ application.EventType, entityType string, entityID uuid.UUID, payload map[string]any) {
	actorID := actor.ID
	s.deps.Bus.Publish(ctx, &application.Event{
		Type:       typ,
		OrgID:      actor.OrgID,
		EntityType: entityType,
		EntityID:   entityID.String(),
		ActorID:    &actorID,
		Payload:    payload,
	})
}

func answerMentionsPerson(learning ai.LearningResult, people []ai.PersonInfo, answer string) bool {
	answerLower := strings.ToLower(answer)
	for _, p := range people {
		if p.ID.String() != deref(learning.PersonID) {
			continue
		}
		nameLower := strings.ToLower(learning.PersonName)
		tokens := strings.Fields(nameLower)
		for _, t := range append([]string{nameLower}, tokens...) {
			if len(t) >= 3 && strings.Contains(answerLower, t) {
				return true
			}
		}
		return false
	}
	return false
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
