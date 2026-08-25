package ai

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"

	"github.com/apa/backend/internal/domain/task"
)

type Orchestrator struct {
	Intent       IntentAgent
	Context      *ContextAgent
	Planner      PlanningAgent
	Questions    QuestionAgent
	Assignment   AssignmentAgent
	Verification VerificationAgent
	Learning     LearningAgent
	log          *slog.Logger
}

func NewOrchestrator(
	intent IntentAgent,
	contextAgent *ContextAgent,
	planner PlanningAgent,
	questions QuestionAgent,
	assignment AssignmentAgent,
	verification VerificationAgent,
	learning LearningAgent,
	log *slog.Logger,
) *Orchestrator {
	return &Orchestrator{
		Intent:       intent,
		Context:      contextAgent,
		Planner:      planner,
		Questions:    questions,
		Assignment:   assignment,
		Verification: verification,
		Learning:     learning,
		log:          log,
	}
}

type PlanOutcome struct {
	Intent      IntentResult               `json:"intent"`
	Context     *OrgContext                `json:"-"`
	Plan        PlanResult                 `json:"plan"`
	Gaps        []Gap                      `json:"gaps,omitempty"`
	Assignments map[int]AssignmentProposal `json:"assignments,omitempty"`
}

func (o *Orchestrator) PlanWorkflow(ctx context.Context, orgID uuid.UUID, intentText string) (*PlanOutcome, error) {
	outcome := &PlanOutcome{}

	g, gCtx := errgroup.WithContext(ctx)
	g.Go(func() error {
		intent, err := o.Intent.AnalyzeIntent(gCtx, intentText)
		if err != nil {
			return fmt.Errorf("intent agent: %w", err)
		}
		outcome.Intent = intent
		return nil
	})
	g.Go(func() error {
		orgCtx, err := o.Context.Gather(gCtx, orgID)
		if err != nil {
			return fmt.Errorf("context agent: %w", err)
		}
		outcome.Context = orgCtx
		return nil
	})
	if err := g.Wait(); err != nil {
		return nil, err
	}

	plan, err := o.Planner.ProposePlan(ctx, outcome.Intent, outcome.Context)
	if err != nil {
		return nil, fmt.Errorf("planning agent: %w", err)
	}
	outcome.Plan = plan

	gaps, err := o.Questions.IdentifyGaps(ctx, outcome.Intent, plan, outcome.Context)
	if err != nil {
		o.log.WarnContext(ctx, "question agent failed", slog.Any("error", err))
	}
	outcome.Gaps = gaps

	assignments, err := o.Assignment.ProposeAssignments(ctx, plan.Tasks, outcome.Context)
	if err != nil {
		o.log.WarnContext(ctx, "assignment agent failed", slog.Any("error", err))
	}
	outcome.Assignments = assignments

	o.log.InfoContext(ctx, "workflow planned",
		slog.String("org_id", orgID.String()),
		slog.String("title", plan.Title),
		slog.Int("tasks", len(plan.Tasks)),
		slog.Int("gaps", len(gaps)),
		slog.Int("assignments", len(assignments)),
	)
	return outcome, nil
}

func TaskProposalFromDomain(t *task.Task) TaskProposal {
	deps := make([]int, 0, len(t.DependsOn))
	return TaskProposal{
		Title:          t.Title,
		Description:    t.Description,
		Topic:          t.Topic,
		RequiredSkills: t.RequiredSkills,
		Dependencies:   deps,
		ExpectedOutput: t.ExpectedOutput,
	}
}

func (o *Orchestrator) ReassignTask(ctx context.Context, orgID uuid.UUID, t *task.Task) (AssignmentProposal, error) {
	orgCtx, err := o.Context.Gather(ctx, orgID)
	if err != nil {
		return AssignmentProposal{}, fmt.Errorf("context agent: %w", err)
	}
	tp := TaskProposalFromDomain(t)
	results, err := o.Assignment.ProposeAssignments(ctx, []TaskProposal{tp}, orgCtx)
	if err != nil {
		return AssignmentProposal{}, fmt.Errorf("assignment agent: %w", err)
	}
	proposal, ok := results[0]
	if !ok {
		return AssignmentProposal{}, fmt.Errorf("%w: assignment agent returned no proposal", ErrInvalidLLMOutput)
	}
	return proposal, nil
}

func (o *Orchestrator) Verify(ctx context.Context, t *task.Task) (VerificationResult, error) {
	return o.Verification.VerifyCompletion(ctx, t.Title, t.CompletedNotes)
}

func (o *Orchestrator) LearnFromAnswer(ctx context.Context, answer string, topic string, people []PersonInfo) (LearningResult, error) {
	return o.Learning.ExtractKnowledge(ctx, answer, topic, people)
}
