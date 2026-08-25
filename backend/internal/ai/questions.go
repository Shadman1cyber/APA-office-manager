package ai

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

const OwnerKnownThreshold = 0.35
const MaxQuestionsPerPlan = 3

type QuestionAgent interface {
	IdentifyGaps(ctx context.Context, intent IntentResult, plan PlanResult, org *OrgContext) ([]Gap, error)
}

func NewQuestionAgent(p LLMProvider) QuestionAgent {
	if _, ok := p.(*MockProvider); ok {
		return &mockQuestionAgent{}
	}
	return &llmQuestionAgent{provider: p}
}

type mockQuestionAgent struct{}

func (m *mockQuestionAgent) IdentifyGaps(ctx context.Context, intent IntentResult, plan PlanResult, org *OrgContext) ([]Gap, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	gaps := []Gap{}

	for i := range plan.Tasks {
		tp := plan.Tasks[i]
		if strings.TrimSpace(tp.Topic) == "" {
			continue
		}
		key := strings.ToLower(tp.Topic)
		if seen[key] {
			continue
		}
		if fact := org.FactFor(tp.Topic); fact != nil && fact.Confidence >= OwnerKnownThreshold {
			seen[key] = true
			continue
		}
		seen[key] = true
		gaps = append(gaps, Gap{
			TaskIndex: i,
			Question: ClarificationQuestion{
				Question: fmt.Sprintf("نمی‌دانم مسئول «%s» چه کسی است. آن را به چه کسی بسپارم؟", tp.Topic),
				Reason:   fmt.Sprintf("هنوز هیچ دانش سازمانی، فردی را به «%s» پیوند نداده است.", tp.Topic),
				Required: true,
				Topic:    tp.Topic,
			},
		})
		if len(gaps) >= MaxQuestionsPerPlan {
			break
		}
	}
	return gaps, nil
}

const questionsSchema = `{"questions":[{"question":"the clarifying question","reason":"why this is needed","required":true,"topic":"topic name","task_index":0}]}`

type llmQuestionWire struct {
	Question  string `json:"question"`
	Reason    string `json:"reason"`
	Required  bool   `json:"required"`
	Topic     string `json:"topic"`
	TaskIndex int    `json:"task_index"`
}

type llmQuestionsWire struct {
	Questions []llmQuestionWire `json:"questions"`
}

type llmQuestionAgent struct {
	provider LLMProvider
}

func (l *llmQuestionAgent) IdentifyGaps(ctx context.Context, intent IntentResult, plan PlanResult, org *OrgContext) ([]Gap, error) {
	instruction := fmt.Sprintf(
		"The team knows the following ownership:\n%s\nGiven this plan:\n%s\nAsk only about missing owners that block the plan.",
		describeFacts(org), describeTasks(plan.Tasks),
	)
	var wire llmQuestionsWire
	if err := l.provider.GenerateStructured(ctx, StructuredRequest{
		Instruction: instruction,
		SchemaHint:  questionsSchema,
	}, &wire); err != nil {
		return nil, err
	}

	gaps := []Gap{}
	for _, q := range wire.Questions {
		if q.TaskIndex < 0 || q.TaskIndex >= len(plan.Tasks) {
			continue
		}
		gaps = append(gaps, Gap{
			TaskIndex: q.TaskIndex,
			Question: ClarificationQuestion{
				Question: q.Question,
				Reason:   q.Reason,
				Required: q.Required,
				Topic:    q.Topic,
			},
		})
		if len(gaps) >= MaxQuestionsPerPlan {
			break
		}
	}
	return gaps, nil
}

func describeTasks(tasks []TaskProposal) string {
	out := ""
	for i, tp := range tasks {
		out += fmt.Sprintf("- [%d] %s (topic: %q, skills: %v)\n", i, tp.Title, tp.Topic, tp.RequiredSkills)
	}
	return out
}

func describeFacts(org *OrgContext) string {
	facts := make([]string, 0, len(org.Facts))
	for _, f := range org.Facts {
		name := f.Subject
		for _, p := range org.People {
			if p.ID == f.PersonID {
				name = fmt.Sprintf("%s -> %s", f.Subject, p.Name)
			}
		}
		facts = append(facts, fmt.Sprintf("- %s (confidence %.2f)", name, f.Confidence))
	}
	sort.Strings(facts)
	if len(facts) == 0 {
		return "- none recorded yet"
	}
	return strings.Join(facts, "\n")
}
