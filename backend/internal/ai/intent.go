package ai

import (
	"context"
	"fmt"
	"strings"

	"github.com/apa/backend/internal/domain"
)

type IntentAgent interface {
	AnalyzeIntent(ctx context.Context, text string) (IntentResult, error)
}

func NewIntentAgent(p LLMProvider) IntentAgent {
	if _, ok := p.(*MockProvider); ok {
		return &mockIntentAgent{}
	}
	return &llmIntentAgent{provider: p}
}

type mockIntentAgent struct{}

func (m *mockIntentAgent) AnalyzeIntent(ctx context.Context, text string) (IntentResult, error) {
	if err := ctx.Err(); err != nil {
		return IntentResult{}, err
	}
	goal := strings.TrimSpace(text)
	if goal == "" {
		return IntentResult{}, fmt.Errorf("%w: empty manager request", domain.ErrInsufficientData)
	}
	goal = strings.TrimSuffix(goal, ".")
	kind := ClassifyIntent(goal)
	if kind == IntentKindSmallTalk {
		return IntentResult{Kind: kind, Goal: strings.TrimSpace(text)}, nil
	}
	return IntentResult{
		Kind:   kind,
		Title:  UpperFirst(goal),
		Goal:   goal,
		Topics: DetectTopics(goal),
	}, nil
}

const intentSchema = `{"kind":"create_report | general_task | smalltalk","title":"short imperative title (empty for smalltalk)","goal":"one sentence restating the goal","topics":["topic-name"]}`

type llmIntentAgent struct {
	provider LLMProvider
}

func (l *llmIntentAgent) AnalyzeIntent(ctx context.Context, text string) (IntentResult, error) {
	var result IntentResult
	err := l.provider.GenerateStructured(ctx, StructuredRequest{
		Instruction: fmt.Sprintf(
			"Classify this manager request and extract its goal and organizational topics: %q\n"+
				"IMPORTANT: greetings, thanks, and small talk that are NOT work requests must use kind=\"smalltalk\" with an empty title.",
			text),
		SchemaHint: intentSchema,
	}, &result)
	if err != nil {
		return IntentResult{}, err
	}
	result.Goal = strings.TrimSpace(result.Goal)
	switch result.Kind {
	case IntentKindSmallTalk:
		if result.Goal == "" {
			result.Goal = strings.TrimSpace(text)
		}
		result.Title = ""
		result.Topics = nil
		return result, nil
	case IntentKindReport, IntentKindGeneral:
	default:
		result.Kind = IntentKindGeneral
	}
	if result.Goal == "" {
		return IntentResult{}, fmt.Errorf("%w: model returned an empty goal", ErrInvalidLLMOutput)
	}
	result.Title = firstNonEmpty(strings.TrimSpace(result.Title), UpperFirst(result.Goal))
	return result, nil
}
