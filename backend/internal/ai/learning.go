package ai

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

const VerificationReinforceDelta = 0.08

type LearningAgent interface {
	ExtractKnowledge(ctx context.Context, answer string, topic string, people []PersonInfo) (LearningResult, error)
}

func NewLearningAgent(p LLMProvider) LearningAgent {
	if _, ok := p.(*MockProvider); ok {
		return &mockLearningAgent{}
	}
	return &llmLearningAgent{provider: p}
}

type mockLearningAgent struct{}

func (m *mockLearningAgent) ExtractKnowledge(ctx context.Context, answer string, topic string, people []PersonInfo) (LearningResult, error) {
	if err := ctx.Err(); err != nil {
		return LearningResult{}, err
	}
	lower := strings.ToLower(answer)

	type hit struct {
		person PersonInfo
		index  int
	}
	var hits []hit

	for _, p := range people {
		full := strings.ToLower(p.Name)
		if idx := strings.Index(lower, full); idx >= 0 {
			hits = append(hits, hit{person: p, index: idx})
			continue
		}
		tokens := strings.Fields(full)
		if len(tokens) == 0 {
			continue
		}
		first := tokens[0]
		for _, word := range strings.Fields(lower) {
			word = strings.Trim(word, ".,!?;:'\"()")
			if word == first {
				hits = append(hits, hit{person: p, index: strings.Index(lower, word)})
				break
			}
		}
	}

	if len(hits) == 0 {
		return LearningResult{Topic: topic}, nil
	}
	sort.Slice(hits, func(i, j int) bool { return hits[i].index < hits[j].index })

	person := hits[0].person
	id := person.ID.String()
	return LearningResult{
		Topic:           topic,
		PersonID:        &id,
		PersonName:      person.Name,
		ConfidenceDelta: 0.08,
		Summary:         fmt.Sprintf("ثبت شد که %s مسئول «%s» است.", person.Name, topic),
	}, nil
}

const learningSchema = `{"topic":"the topic the answer clarifies","person_id":"uuid of mentioned teammate or null","person_name":"their name or empty","confidence_delta":0.08,"summary":"one line describing the learned fact"}`

type llmLearningAgent struct {
	provider LLMProvider
}

func (l *llmLearningAgent) ExtractKnowledge(ctx context.Context, answer string, topic string, people []PersonInfo) (LearningResult, error) {
	var result LearningResult
	err := l.provider.GenerateStructured(ctx, StructuredRequest{
		Instruction: fmt.Sprintf(
			"The manager answered a clarification question about topic %q with: %q\nMap the answer onto one of these teammates (id): %s",
			topic, answer, describePeopleIDs(&OrgContext{People: people}),
		),
		SchemaHint: learningSchema,
	}, &result)
	if err != nil {
		return LearningResult{}, err
	}
	if result.PersonID != nil && result.PersonName == "" {
		for _, p := range people {
			if p.ID.String() == *result.PersonID {
				result.PersonName = p.Name
			}
		}
	}
	result.ConfidenceDelta = clamp01(result.ConfidenceDelta)
	return result, nil
}
