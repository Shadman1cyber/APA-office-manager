package workflowsvc

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"

	"github.com/apa/backend/internal/ai"
	"github.com/apa/backend/internal/application"
	"github.com/apa/backend/internal/application/approval"
	"github.com/apa/backend/internal/application/assignment"
	"github.com/apa/backend/internal/domain"
	"github.com/apa/backend/internal/domain/approval"
	"github.com/apa/backend/internal/domain/question"
	"github.com/apa/backend/internal/domain/task"
	"github.com/apa/backend/internal/domain/user"
	"github.com/apa/backend/internal/domain/workflow"
)

type Deps struct {
	Workflows    application.WorkflowRepository
	Tasks        application.TaskRepository
	Questions    application.QuestionRepository
	Approvals    *approvalsvc.Service
	Users        application.UserRepository
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

func (s *Service) Create(ctx context.Context, actor *user.User, intentText string) (*View, error) {
	intentText = strings.TrimSpace(intentText)
	if intentText == "" {
		return nil, domain.Invalid("intent", "در یک جمله بگویید چه کاری نیاز دارید")
	}
	if len(intentText) > 2000 {
		return nil, domain.Invalid("intent", "متن درخواست بیش از حد طولانی است (حداکثر ۲۰۰۰ نویسه)")
	}
	if !actor.Role.CanApprove() {
		return nil, fmt.Errorf("%w: ثبت درخواست جدید فقط برای مدیران فعال است", domain.ErrForbidden)
	}

	outcome, err := s.deps.Orchestrator.PlanWorkflow(ctx, actor.OrgID, intentText)
	if err != nil {
		return nil, err
	}
	if outcome.Intent.Kind == ai.IntentKindSmallTalk {
		return nil, fmt.Errorf("%w: that doesn't look like a task", ai.ErrSmallTalk)
	}
	return s.persistPlan(ctx, actor, intentText, outcome)
}

func (s *Service) persistPlan(ctx context.Context, actor *user.User, intentText string, outcome *ai.PlanOutcome) (*View, error) {
	wf, err := workflow.New(actor.OrgID, actor.ID, outcome.Plan.Title, intentText)
	if err != nil {
		return nil, err
	}
	if err := s.deps.Workflows.Create(ctx, wf); err != nil {
		return nil, fmt.Errorf("create workflow: %w", err)
	}
	s.publish(ctx, actor, application.EventWorkflowCreated, "workflow", wf.ID, map[string]any{
		"title": wf.Title,
	})

	tasksList := make([]*task.Task, len(outcome.Plan.Tasks))
	for i, tp := range outcome.Plan.Tasks {
		tasksList[i] = &task.Task{
			OrgID:          actor.OrgID,
			WorkflowID:     wf.ID,
			Position:       i,
			Title:          tp.Title,
			Description:    tp.Description,
			Topic:          tp.Topic,
			RequiredSkills: tp.RequiredSkills,
			ExpectedOutput: tp.ExpectedOutput,
			Status:         task.StatusProposed,
		}
	}
	if len(tasksList) == 0 {
		return nil, fmt.Errorf("%w: plan produced no tasks", domain.ErrInsufficientData)
	}
	if err := s.deps.Tasks.CreateBatch(ctx, tasksList); err != nil {
		return nil, fmt.Errorf("create plan tasks: %w", err)
	}

	depMap := make(map[uuid.UUID][]uuid.UUID)
	for i, tp := range outcome.Plan.Tasks {
		for _, depIdx := range tp.Dependencies {
			if depIdx < 0 || depIdx >= len(tasksList) || depIdx == i {
				continue
			}
			depMap[tasksList[i].ID] = append(depMap[tasksList[i].ID], tasksList[depIdx].ID)
		}
	}
	if err := s.deps.Tasks.SetDependencies(ctx, depMap); err != nil {
		return nil, fmt.Errorf("link task dependencies: %w", err)
	}

	proposedOwners := []string{}
	for i, ap := range outcome.Assignments {
		if i < 0 || i >= len(tasksList) {
			continue
		}
		proposal, err := assignmentsvc.FromAI(ap)
		if err != nil {
			s.deps.Log.WarnContext(ctx, "discarding invalid assignment proposal",
				slog.Int("task_index", i), slog.Any("error", err))
			continue
		}
		if err := s.deps.Tasks.SaveProposal(ctx, tasksList[i].ID, proposal); err != nil {
			return nil, fmt.Errorf("save assignment proposal: %w", err)
		}
		payload := map[string]any{
			"task_id":    tasksList[i].ID.String(),
			"task_title": tasksList[i].Title,
			"confidence": proposal.Confidence,
			"evidence":   evidenceToAny(proposal.Evidence),
			"candidate":  candidateName(proposal),
		}
		s.publish(ctx, actor, application.EventAssignmentProposed, "task", tasksList[i].ID, payload)
		if proposal.CandidateName != "" {
			proposedOwners = append(proposedOwners,
				fmt.Sprintf("%s → %s (%.0f%% confidence)", tasksList[i].Title, proposal.CandidateName, proposal.Confidence*100))
		}
	}

	planPayload, err := json.Marshal(map[string]any{
		"title":       outcome.Plan.Title,
		"rationale":   outcome.Plan.Rationale,
		"tasks":       outcome.Plan.Tasks,
		"assignments": outcome.Assignments,
	})
	if err != nil {
		return nil, fmt.Errorf("serialize plan: %w", err)
	}
	if _, err := s.deps.Approvals.CreatePlanApproval(ctx, actor.OrgID, wf.ID, planPayload); err != nil {
		return nil, fmt.Errorf("create approval: %w", err)
	}

	askedTopics := map[string]bool{}
	for _, gap := range outcome.Gaps {
		topicKey := strings.ToLower(gap.Question.Topic)
		if askedTopics[topicKey] {
			continue
		}
		askedTopics[topicKey] = true

		q := &question.Question{
			OrgID:      actor.OrgID,
			WorkflowID: wf.ID,
			TaskIndex:  gap.TaskIndex,
			Topic:      gap.Question.Topic,
			Text:       gap.Question.Question,
			Reason:     gap.Question.Reason,
			Required:   gap.Question.Required,
			Status:     question.StatusOpen,
		}
		if gap.TaskIndex >= 0 && gap.TaskIndex < len(tasksList) {
			taskID := tasksList[gap.TaskIndex].ID
			q.RelatedTaskID = &taskID
		}
		if err := s.deps.Questions.Create(ctx, q); err != nil {
			return nil, fmt.Errorf("create clarification question: %w", err)
		}
		s.publish(ctx, actor, application.EventQuestionCreated, "question", q.ID, map[string]any{
			"workflow_id": wf.ID.String(),
			"question":    q.Text,
			"topic":       q.Topic,
		})
	}

	s.publish(ctx, actor, application.EventPlanGenerated, "workflow", wf.ID, map[string]any{
		"task_count": len(tasksList),
	})

	view, err := s.Get(ctx, actor.OrgID, wf.ID)
	if err != nil {
		return nil, err
	}
	view.ProposedOwners = proposedOwners
	return view, nil
}

func (s *Service) Get(ctx context.Context, orgID, id uuid.UUID) (*View, error) {
	wf, err := s.deps.Workflows.Get(ctx, orgID, id)
	if err != nil {
		return nil, err
	}
	tasksList, err := s.deps.Tasks.ListByWorkflow(ctx, id)
	if err != nil {
		return nil, err
	}
	qs, err := s.deps.Questions.ListByWorkflow(ctx, id)
	if err != nil {
		return nil, err
	}
	return &View{Workflow: wf, Tasks: tasksList, Questions: qs}, nil
}

func (s *Service) List(ctx context.Context, orgID uuid.UUID, limit int) ([]*workflow.Workflow, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	return s.deps.Workflows.List(ctx, orgID, limit)
}

func (s *Service) ListForUser(ctx context.Context, orgID, userID uuid.UUID) ([]*workflow.Workflow, error) {
	return s.deps.Workflows.ListForUser(ctx, orgID, userID)
}

func (s *Service) Approve(ctx context.Context, actor *user.User, id uuid.UUID) (*View, []string, error) {
	if !actor.Role.CanApprove() {
		return nil, nil, fmt.Errorf("%w: فقط مدیران می‌توانند گردش‌کار را تأیید کنند", domain.ErrForbidden)
	}
	wf, err := s.deps.Workflows.Get(ctx, actor.OrgID, id)
	if err != nil {
		return nil, nil, err
	}

	openRequired, err := s.deps.Questions.ListOpenRequired(ctx, wf.ID)
	if err != nil {
		return nil, nil, err
	}
	if len(openRequired) > 0 {
		texts := make([]string, len(openRequired))
		for i, q := range openRequired {
			texts[i] = q.Text
		}
		return nil, nil, fmt.Errorf(
			"%w: پیش از تأیید باید به %d سؤال الزامی پاسخ داده شود: %s",
			domain.ErrInvalidState, len(openRequired), strings.Join(texts, " | "))
	}

	appr, err := s.deps.Approvals.LatestPlan(ctx, wf.ID)
	if err != nil {
		return nil, nil, err
	}
	if appr.Status != approval.StatusPending {
		return nil, nil, fmt.Errorf("%w: plan approval is already %s", domain.ErrInvalidState, appr.Status)
	}
	if err := wf.TransitionTo(workflow.StatusApproved); err != nil {
		return nil, nil, err
	}

	tasksList, err := s.deps.Tasks.ListByWorkflow(ctx, wf.ID)
	if err != nil {
		return nil, nil, err
	}

	var assignedSummaries []string
	for _, t := range tasksList {
		if t.Proposal != nil && t.Proposal.CandidateUserID != nil {
			cand, err := s.deps.Users.Get(ctx, *t.Proposal.CandidateUserID)
			if err != nil || cand.OrgID != actor.OrgID {
				s.deps.Log.WarnContext(ctx, "assignment candidate invalid; task left unassigned",
					slog.String("task_id", t.ID.String()))
				continue
			}
			if err := s.deps.Tasks.Assign(ctx, t.ID, &cand.ID, t.Status, task.StatusAssigned); err != nil {
				return nil, nil, err
			}
			s.publish(ctx, actor, application.EventAssignmentApproved, "task", t.ID, map[string]any{
				"assignee": cand.Name,
			})
			s.publish(ctx, actor, application.EventTaskAssigned, "task", t.ID, map[string]any{
				"assignee":    cand.Name,
				"workflow_id": wf.ID.String(),
			})
			assignedSummaries = append(assignedSummaries, fmt.Sprintf("%s → %s", t.Title, cand.Name))
		} else if t.Status == task.StatusProposed {
			if err := s.deps.Tasks.UpdateStatusExpected(ctx, t.ID, task.StatusProposed, task.StatusPending); err != nil {
				return nil, nil, err
			}
		}
	}

	if err := s.deps.Workflows.UpdateStatusExpected(ctx, actor.OrgID, wf.ID, workflow.StatusProposed, workflow.StatusApproved); err != nil {
		return nil, nil, err
	}
	if err := s.deps.Approvals.Decide(ctx, appr.ID, approval.StatusApproved, actor.ID); err != nil {
		return nil, nil, err
	}
	s.publish(ctx, actor, application.EventWorkflowApproved, "workflow", wf.ID, map[string]any{
		"title": wf.Title,
	})

	view, err := s.Get(ctx, actor.OrgID, wf.ID)
	if err != nil {
		return nil, nil, err
	}
	return view, assignedSummaries, nil
}

func (s *Service) Reject(ctx context.Context, actor *user.User, id uuid.UUID, reason string) (*View, error) {
	if !actor.Role.CanApprove() {
		return nil, fmt.Errorf("%w: فقط مدیران می‌توانند گردش‌کار را رد کنند", domain.ErrForbidden)
	}
	wf, err := s.deps.Workflows.Get(ctx, actor.OrgID, id)
	if err != nil {
		return nil, err
	}
	if err := wf.TransitionTo(workflow.StatusRejected); err != nil {
		return nil, err
	}
	appr, err := s.deps.Approvals.LatestPlan(ctx, wf.ID)
	if err != nil {
		return nil, err
	}
	if appr.Status == approval.StatusPending {
		if err := s.deps.Approvals.Decide(ctx, appr.ID, approval.StatusRejected, actor.ID); err != nil {
			return nil, err
		}
	}
	if err := s.deps.Workflows.UpdateStatusExpected(ctx, actor.OrgID, wf.ID, workflow.StatusProposed, workflow.StatusRejected); err != nil {
		return nil, err
	}
	payload := map[string]any{"title": wf.Title}
	if reason != "" {
		payload["reason"] = reason
	}
	s.publish(ctx, actor, application.EventWorkflowRejected, "workflow", wf.ID, payload)

	return s.Get(ctx, actor.OrgID, wf.ID)
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

func candidateName(p *task.Proposal) any {
	if p == nil || p.CandidateName == "" {
		return nil
	}
	return p.CandidateName
}

func evidenceToAny(ev []string) []string {
	if ev == nil {
		return []string{}
	}
	return ev
}
