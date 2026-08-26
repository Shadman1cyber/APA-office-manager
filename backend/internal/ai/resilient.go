package ai

import (
	"context"
	"log/slog"
)

type ResilientIntentAgent struct {
	primary  IntentAgent
	fallback IntentAgent
	log      *slog.Logger
}

func NewResilientIntentAgent(primary, fallback IntentAgent, log *slog.Logger) *ResilientIntentAgent {
	return &ResilientIntentAgent{primary: primary, fallback: fallback, log: log}
}

func (r *ResilientIntentAgent) AnalyzeIntent(ctx context.Context, text string) (IntentResult, error) {
	result, err := r.primary.AnalyzeIntent(ctx, text)
	if err != nil {
		r.log.WarnContext(ctx, "intent agent fell back to deterministic mode", slog.Any("error", err))
		return r.fallback.AnalyzeIntent(ctx, text)
	}
	return result, nil
}

type ResilientPlanningAgent struct {
	primary  PlanningAgent
	fallback PlanningAgent
	log      *slog.Logger
}

func NewResilientPlanningAgent(primary, fallback PlanningAgent, log *slog.Logger) *ResilientPlanningAgent {
	return &ResilientPlanningAgent{primary: primary, fallback: fallback, log: log}
}

func (r *ResilientPlanningAgent) ProposePlan(ctx context.Context, intent IntentResult, org *OrgContext) (PlanResult, error) {
	result, err := r.primary.ProposePlan(ctx, intent, org)
	if err != nil {
		r.log.WarnContext(ctx, "planning agent fell back to deterministic mode", slog.Any("error", err))
		return r.fallback.ProposePlan(ctx, intent, org)
	}
	return result, nil
}

type ResilientQuestionAgent struct {
	primary  QuestionAgent
	fallback QuestionAgent
	log      *slog.Logger
}

func NewResilientQuestionAgent(primary, fallback QuestionAgent, log *slog.Logger) *ResilientQuestionAgent {
	return &ResilientQuestionAgent{primary: primary, fallback: fallback, log: log}
}

func (r *ResilientQuestionAgent) IdentifyGaps(ctx context.Context, intent IntentResult, plan PlanResult, org *OrgContext) ([]Gap, error) {
	gaps, err := r.primary.IdentifyGaps(ctx, intent, plan, org)
	if err != nil {
		r.log.WarnContext(ctx, "question agent fell back to deterministic mode", slog.Any("error", err))
		return r.fallback.IdentifyGaps(ctx, intent, plan, org)
	}
	return gaps, nil
}

type ResilientAssignmentAgent struct {
	primary  AssignmentAgent
	fallback AssignmentAgent
	log      *slog.Logger
}

func NewResilientAssignmentAgent(primary, fallback AssignmentAgent, log *slog.Logger) *ResilientAssignmentAgent {
	return &ResilientAssignmentAgent{primary: primary, fallback: fallback, log: log}
}

func (r *ResilientAssignmentAgent) ProposeAssignments(ctx context.Context, tasks []TaskProposal, org *OrgContext) (map[int]AssignmentProposal, error) {
	results, err := r.primary.ProposeAssignments(ctx, tasks, org)
	if err != nil || len(results) == 0 {
		if err != nil {
			r.log.WarnContext(ctx, "assignment agent fell back to deterministic mode", slog.Any("error", err))
		} else {
			r.log.WarnContext(ctx, "assignment agent returned no proposals; falling back")
		}
		return r.fallback.ProposeAssignments(ctx, tasks, org)
	}
	return results, nil
}

type ResilientVerificationAgent struct {
	primary  VerificationAgent
	fallback VerificationAgent
	log      *slog.Logger
}

func NewResilientVerificationAgent(primary, fallback VerificationAgent, log *slog.Logger) *ResilientVerificationAgent {
	return &ResilientVerificationAgent{primary: primary, fallback: fallback, log: log}
}

func (r *ResilientVerificationAgent) VerifyCompletion(ctx context.Context, taskTitle string, notes string) (VerificationResult, error) {
	result, err := r.primary.VerifyCompletion(ctx, taskTitle, notes)
	if err != nil {
		r.log.WarnContext(ctx, "verification agent fell back to deterministic mode", slog.Any("error", err))
		return r.fallback.VerifyCompletion(ctx, taskTitle, notes)
	}
	return result, nil
}

type ResilientLearningAgent struct {
	primary  LearningAgent
	fallback LearningAgent
	log      *slog.Logger
}

func NewResilientLearningAgent(primary, fallback LearningAgent, log *slog.Logger) *ResilientLearningAgent {
	return &ResilientLearningAgent{primary: primary, fallback: fallback, log: log}
}

func (r *ResilientLearningAgent) ExtractKnowledge(ctx context.Context, answer string, topic string, people []PersonInfo) (LearningResult, error) {
	result, err := r.primary.ExtractKnowledge(ctx, answer, topic, people)
	if err != nil {
		r.log.WarnContext(ctx, "learning agent fell back to deterministic mode", slog.Any("error", err))
		return r.fallback.ExtractKnowledge(ctx, answer, topic, people)
	}
	return result, nil
}

type ResilientDocumentationAgent struct {
	primary  DocumentationAgent
	fallback DocumentationAgent
	log      *slog.Logger
}

func NewResilientDocumentationAgent(primary, fallback DocumentationAgent, log *slog.Logger) *ResilientDocumentationAgent {
	return &ResilientDocumentationAgent{primary: primary, fallback: fallback, log: log}
}

func (r *ResilientDocumentationAgent) GenerateDocument(ctx context.Context, in DocumentInput) (GeneratedDocument, error) {
	result, err := r.primary.GenerateDocument(ctx, in)
	if err != nil {
		r.log.WarnContext(ctx, "documentation agent fell back to deterministic mode", slog.Any("error", err))
		return r.fallback.GenerateDocument(ctx, in)
	}
	return result, nil
}

type ResilientDocGuardAgent struct {
	primary  DocGuardAgent
	fallback DocGuardAgent
	log      *slog.Logger
}

func NewResilientDocGuardAgent(primary, fallback DocGuardAgent, log *slog.Logger) *ResilientDocGuardAgent {
	return &ResilientDocGuardAgent{primary: primary, fallback: fallback, log: log}
}

func (r *ResilientDocGuardAgent) CheckNotes(ctx context.Context, authorName string, notes string) (NoteVerdict, error) {
	verdict, err := r.primary.CheckNotes(ctx, authorName, notes)
	if err != nil {
		r.log.WarnContext(ctx, "doc notes guard fell back to deterministic mode", slog.Any("error", err))
		return r.fallback.CheckNotes(ctx, authorName, notes)
	}
	return verdict, nil
}

func (r *ResilientDocGuardAgent) CheckDocument(ctx context.Context, in DocumentInput, body string) (DocVerdict, error) {
	verdict, err := r.primary.CheckDocument(ctx, in, body)
	if err != nil {
		r.log.WarnContext(ctx, "doc coherence guard fell back to deterministic mode", slog.Any("error", err))
		return r.fallback.CheckDocument(ctx, in, body)
	}
	return verdict, nil
}
