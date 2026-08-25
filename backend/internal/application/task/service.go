package tasksvc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/apa/backend/internal/ai"
	"github.com/apa/backend/internal/application"
	approvalsvc "github.com/apa/backend/internal/application/approval"
	knowledgesvc "github.com/apa/backend/internal/application/knowledge"
	"github.com/apa/backend/internal/domain"
	domaintask "github.com/apa/backend/internal/domain/task"
	"github.com/apa/backend/internal/domain/user"
	"github.com/apa/backend/internal/domain/workflow"
)

const VerifyJobType = "verify_task"

type Deps struct {
	Tasks        application.TaskRepository
	Workflows    application.WorkflowRepository
	Users        application.UserRepository
	Knowledge    *knowledgesvc.Service
	Approvals    *approvalsvc.Service
	Orchestrator *ai.Orchestrator
	Bus          *application.Bus
	Jobs         application.JobEnqueuer
	Log          *slog.Logger
}

type Service struct {
	deps Deps
}

func NewService(deps Deps) *Service {
	return &Service{deps: deps}
}

func (s *Service) Get(ctx context.Context, orgID, id uuid.UUID) (*domaintask.Task, error) {
	return s.deps.Tasks.Get(ctx, orgID, id)
}

func (s *Service) UserInvolvedInWorkflow(ctx context.Context, orgID, workflowID, userID uuid.UUID) (bool, error) {
	return s.deps.Tasks.UserInvolvedInWorkflow(ctx, orgID, workflowID, userID)
}

func (s *Service) List(ctx context.Context, actor *user.User, assignedToMe bool, status string, limit int) ([]*domaintask.Task, error) {
	var assignee *uuid.UUID
	if assignedToMe {
		assignee = &actor.ID
	}
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	return s.deps.Tasks.ListByOrg(ctx, actor.OrgID, assignee, status, limit)
}

func (s *Service) ListAvailable(ctx context.Context, actor *user.User, limit int) ([]*domaintask.Task, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	return s.deps.Tasks.ListAvailable(ctx, actor.OrgID, limit)
}

func (s *Service) Claim(ctx context.Context, actor *user.User, id uuid.UUID) (*domaintask.Task, error) {
	t, err := s.deps.Tasks.Get(ctx, actor.OrgID, id)
	if err != nil {
		return nil, err
	}
	if t.Status != domaintask.StatusPending || t.AssignedTo != nil {
		return nil, fmt.Errorf("%w: این وظیفه دیگر قابل دریافت نیست", domain.ErrInvalidState)
	}
	if err := s.deps.Tasks.Assign(ctx, t.ID, &actor.ID, domaintask.StatusPending, domaintask.StatusAssigned); err != nil {
		return nil, err
	}
	proposal := &domaintask.Proposal{
		CandidateUserID:           &actor.ID,
		CandidateName:             actor.Name,
		Evidence:                  []string{"دریافت خودکار توسط " + actor.Name},
		Confidence:                1,
		RequiresHumanConfirmation: false,
	}
	if err := s.deps.Tasks.SaveProposal(ctx, t.ID, proposal); err != nil {
		return nil, err
	}
	s.publish(ctx, actor, application.EventTaskAssigned, "task", t.ID, map[string]any{
		"assignee":    actor.Name,
		"claimed":     true,
		"workflow_id": t.WorkflowID.String(),
	})
	s.advanceWorkflowStatus(ctx, actor.OrgID, t.WorkflowID)
	return s.deps.Tasks.Get(ctx, actor.OrgID, id)
}

func (s *Service) Start(ctx context.Context, actor *user.User, id uuid.UUID) (*domaintask.Task, error) {
	t, err := s.deps.Tasks.Get(ctx, actor.OrgID, id)
	if err != nil {
		return nil, err
	}
	if !s.canAct(actor, t) {
		return nil, fmt.Errorf("%w: این وظیفه به شخص دیگری تخصیص یافته است", domain.ErrForbidden)
	}
	if t.AssignedTo == nil {
		return nil, fmt.Errorf("%w: قبل از شروع، وظیفه باید تخصیص یابد", domain.ErrInvalidState)
	}
	if err := t.DependenciesSatisfied(func(depID uuid.UUID) (domaintask.Status, error) {
		dep, err := s.deps.Tasks.Get(ctx, actor.OrgID, depID)
		if err != nil {
			return "", err
		}
		return dep.Status, nil
	}); err != nil {
		return nil, err
	}

	if err := t.TransitionTo(domaintask.StatusInProgress); err != nil {
		return nil, err
	}
	if err := s.deps.Tasks.UpdateStatusExpected(ctx, t.ID, domaintask.StatusAssigned, domaintask.StatusInProgress); err != nil {
		return nil, err
	}

	s.advanceWorkflowStatus(ctx, actor.OrgID, t.WorkflowID)
	s.publish(ctx, actor, application.EventTaskStarted, "task", t.ID, map[string]any{
		"workflow_id": t.WorkflowID.String(),
		"title":       t.Title,
	})
	return s.deps.Tasks.Get(ctx, actor.OrgID, id)
}

func (s *Service) Complete(ctx context.Context, actor *user.User, id uuid.UUID, notes string) (*domaintask.Task, error) {
	notes = strings.TrimSpace(notes)
	if len(notes) < 15 {
		return nil, domain.Invalid("notes", "توضیح دهید چه چیزی تحویل دادید (حداقل ۱۵ نویسه)")
	}
	t, err := s.deps.Tasks.Get(ctx, actor.OrgID, id)
	if err != nil {
		return nil, err
	}
	if !s.canAct(actor, t) {
		return nil, fmt.Errorf("%w: این وظیفه به شخص دیگری تخصیص یافته است", domain.ErrForbidden)
	}
	if err := t.TransitionTo(domaintask.StatusCompleted); err != nil {
		return nil, err
	}
	if err := s.deps.Tasks.UpdateStatusExpected(ctx, t.ID, domaintask.StatusInProgress, domaintask.StatusCompleted); err != nil {
		return nil, err
	}
	if err := s.deps.Tasks.SetCompletionNotes(ctx, t.ID, notes); err != nil {
		return nil, err
	}
	s.publish(ctx, actor, application.EventTaskCompleted, "task", t.ID, map[string]any{
		"workflow_id": t.WorkflowID.String(),
		"title":       t.Title,
	})

	payload, _ := json.Marshal(map[string]string{"task_id": t.ID.String(), "org_id": t.OrgID.String()})
	if err := s.deps.Jobs.Enqueue(ctx, VerifyJobType, payload, time.Now().UTC().Add(time.Second)); err != nil {
		s.deps.Log.ErrorContext(ctx, "enqueue verification job failed", slog.Any("error", err))
	}
	return s.deps.Tasks.Get(ctx, actor.OrgID, id)
}

func (s *Service) Resume(ctx context.Context, actor *user.User, id uuid.UUID, guidance string) (*domaintask.Task, error) {
	if !actor.Role.CanApprove() {
		return nil, fmt.Errorf("%w: فقط مدیران می‌توانند وظیفهٔ مسدودشده را ادامه دهند", domain.ErrForbidden)
	}
	guidance = strings.TrimSpace(guidance)
	if guidance == "" {
		return nil, domain.Invalid("guidance", "به تیم بگویید مشکل چگونه برطرف شود")
	}
	t, err := s.deps.Tasks.Get(ctx, actor.OrgID, id)
	if err != nil {
		return nil, err
	}
	if err := t.TransitionTo(domaintask.StatusInProgress); err != nil {
		return nil, err
	}
	notes := strings.TrimSpace(t.CompletedNotes + "\nراهنمایی مدیر: " + guidance)
	if err := s.deps.Tasks.SetCompletionNotes(ctx, t.ID, notes); err != nil {
		return nil, err
	}
	if err := s.deps.Tasks.UpdateStatusExpected(ctx, t.ID, domaintask.StatusBlocked, domaintask.StatusInProgress); err != nil {
		return nil, err
	}
	s.publish(ctx, actor, application.EventTaskStarted, "task", t.ID, map[string]any{
		"workflow_id": t.WorkflowID.String(),
		"resumed":     true,
	})
	return s.deps.Tasks.Get(ctx, actor.OrgID, id)
}

func (s *Service) Assign(ctx context.Context, actor *user.User, id, targetUserID uuid.UUID) (*domaintask.Task, error) {
	if !actor.Role.CanApprove() {
		return nil, fmt.Errorf("%w: فقط مدیران می‌توانند وظایف را دوباره تخصیص دهند", domain.ErrForbidden)
	}
	target, err := s.deps.Users.Get(ctx, targetUserID)
	if err != nil {
		return nil, err
	}
	if target.OrgID != actor.OrgID {
		return nil, fmt.Errorf("%w: این کاربر عضو این سازمان نیست", domain.ErrForbidden)
	}
	t, err := s.deps.Tasks.Get(ctx, actor.OrgID, id)
	if err != nil {
		return nil, err
	}
	switch t.Status {
	case domaintask.StatusProposed, domaintask.StatusPending, domaintask.StatusBlocked:
	default:
		return nil, fmt.Errorf("%w: تخصیص وظیفه در وضعیت %s ممکن نیست", domain.ErrInvalidState, t.Status)
	}
	if err := s.deps.Tasks.Assign(ctx, t.ID, &target.ID, t.Status, domaintask.StatusAssigned); err != nil {
		return nil, err
	}
	proposal := &domaintask.Proposal{
		CandidateUserID:           &target.ID,
		CandidateName:             target.Name,
		Evidence:                  []string{fmt.Sprintf("تخصیص دستی توسط %s", actor.Name)},
		Confidence:                1,
		RequiresHumanConfirmation: false,
	}
	if err := s.deps.Tasks.SaveProposal(ctx, t.ID, proposal); err != nil {
		return nil, err
	}
	s.publish(ctx, actor, application.EventTaskAssigned, "task", t.ID, map[string]any{
		"assignee":    target.Name,
		"manual":      true,
		"workflow_id": t.WorkflowID.String(),
	})
	return s.deps.Tasks.Get(ctx, actor.OrgID, id)
}

func (s *Service) VerifyCompleted(ctx context.Context, orgID, taskID uuid.UUID) error {
	t, err := s.deps.Tasks.Get(ctx, orgID, taskID)
	if err != nil {
		return err
	}
	if t.Status != domaintask.StatusCompleted {
		s.deps.Log.InfoContext(ctx, "verification skipped; task not in completed state",
			slog.String("task_id", t.ID.String()), slog.String("status", string(t.Status)))
		return nil
	}

	result, err := s.deps.Orchestrator.Verify(ctx, t)
	if err != nil {
		return fmt.Errorf("verification agent: %w", err)
	}

	if result.Passed {
		if err := s.deps.Tasks.SetVerified(ctx, t.ID, time.Now().UTC()); err != nil {
			return err
		}
		s.publishSystem(ctx, orgID, application.EventTaskVerified, "task", t.ID, map[string]any{
			"passed":       true,
			"feedback":     result.Feedback,
			"confidence":   result.Confidence,
			"workflow_id":  t.WorkflowID.String(),
			"verified_for": t.Title,
		})

		if t.AssignedTo != nil && strings.TrimSpace(t.Topic) != "" {
			fact, rerr := s.deps.Knowledge.Reinforce(ctx, orgID, t.Topic, *t.AssignedTo,
				ai.VerificationReinforceDelta,
				fmt.Sprintf("تحویل موفق «%s»", t.Title))
			if rerr != nil && !errors.Is(rerr, domain.ErrNotFound) {
				s.deps.Log.WarnContext(ctx, "knowledge reinforcement failed", slog.Any("error", rerr))
			}
			if fact != nil {
				s.publishSystem(ctx, orgID, application.EventKnowledgeLearned, "knowledge", fact.ID, map[string]any{
					"subject":    fact.Subject,
					"person":     personName(t),
					"confidence": fact.Confidence,
					"source":     "completion",
				})
			}
		}

		total, verified, err := s.deps.Tasks.WorkflowProgress(ctx, t.WorkflowID)
		if err != nil {
			return err
		}
		if total > 0 && total == verified {
			wf, err := s.deps.Workflows.Get(ctx, orgID, t.WorkflowID)
			if err != nil {
				return err
			}
			if wf.Status == workflow.StatusInProgress {
				if err := s.deps.Workflows.UpdateStatusExpected(ctx, orgID, wf.ID, workflow.StatusInProgress, workflow.StatusCompleted); err != nil {
					return err
				}
				s.publishSystem(ctx, orgID, application.EventWorkflowCompleted, "workflow", wf.ID, map[string]any{
					"title": wf.Title,
				})
			}
		}
	} else {
		if err := s.deps.Tasks.BlockTask(ctx, t.ID, domaintask.StatusCompleted); err != nil {
			return err
		}
		s.publishSystem(ctx, orgID, application.EventTaskVerified, "task", t.ID, map[string]any{
			"passed":      false,
			"feedback":    result.Feedback,
			"confidence":  result.Confidence,
			"workflow_id": t.WorkflowID.String(),
		})
	}
	return nil
}

func (s *Service) advanceWorkflowStatus(ctx context.Context, orgID, wfID uuid.UUID) {
	wf, err := s.deps.Workflows.Get(ctx, orgID, wfID)
	if err != nil {
		return
	}
	switch wf.Status {
	case workflow.StatusApproved:
		if err := s.deps.Workflows.UpdateStatusExpected(ctx, orgID, wfID, workflow.StatusApproved, workflow.StatusInProgress); err != nil {
			s.deps.Log.WarnContext(ctx, "could not move workflow to in_progress", slog.Any("error", err))
		}
	case workflow.StatusProposed:
		_ = wf
	}
}

func (s *Service) canAct(actor *user.User, t *domaintask.Task) bool {
	if actor.Role.CanApprove() {
		return true
	}
	return t.AssignedTo != nil && *t.AssignedTo == actor.ID
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

func (s *Service) publishSystem(ctx context.Context, orgID uuid.UUID, typ application.EventType, entityType string, entityID uuid.UUID, payload map[string]any) {
	s.deps.Bus.Publish(ctx, &application.Event{
		Type:       typ,
		OrgID:      orgID,
		EntityType: entityType,
		EntityID:   entityID.String(),
		Payload:    payload,
	})
}

func personName(t *domaintask.Task) any {
	if t.AssigneeName != "" {
		return t.AssigneeName
	}
	if t.AssignedTo != nil {
		return t.AssignedTo.String()
	}
	return nil
}
