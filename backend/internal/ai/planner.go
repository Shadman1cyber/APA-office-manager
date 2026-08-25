package ai

import (
	"context"
	"fmt"
	"strings"

	"github.com/apa/backend/internal/domain"
)

type PlanningAgent interface {
	ProposePlan(ctx context.Context, intent IntentResult, org *OrgContext) (PlanResult, error)
}

func NewPlanningAgent(p LLMProvider) PlanningAgent {
	if _, ok := p.(*MockProvider); ok {
		return &mockPlanningAgent{}
	}
	return &llmPlanningAgent{provider: p}
}

var topicSkills = map[string]string{
	"cybersecurity":       "security",
	"incident statistics": "statistics",
	"phishing awareness":  "security",
	"budgeting":           "finance",
	"marketing":           "marketing",
	"employee onboarding": "hr",
	"compliance":          "legal",
	"customer feedback":   "support",
	"sales reporting":     "sales",
}

func skillsForTopic(topic string) string {
	if s, ok := topicSkills[topic]; ok {
		return s
	}
	return ""
}

type mockPlanningAgent struct{}

func (m *mockPlanningAgent) ProposePlan(ctx context.Context, intent IntentResult, org *OrgContext) (PlanResult, error) {
	if err := ctx.Err(); err != nil {
		return PlanResult{}, err
	}
	if intent.Goal == "" {
		return PlanResult{}, fmt.Errorf("%w: cannot plan without a goal", domain.ErrInsufficientData)
	}

	var tasks []TaskProposal
	switch intent.Kind {
	case "create_report":
		tasks = reportTasks(intent)
	default:
		tasks = generalTasks(intent)
	}

	if err := validatePlan(tasks); err != nil {
		return PlanResult{}, err
	}

	return PlanResult{
		Title:     intent.Title,
		Rationale: fmt.Sprintf("درخواست به %d گامِ هم‌راستا با شیوهٔ معمول این تیم در تولید خروجی شکسته شد.", len(tasks)),
		Tasks:     tasks,
	}, nil
}

func reportTasks(intent IntentResult) []TaskProposal {
	primary := firstNonEmpty(intent.Topics...)
	subject := primary
	if subject == "" {
		subject = "موضوع درخواستی"
	}
	collectSkills := []string{"research"}
	if s := skillsForTopic(primary); s != "" {
		collectSkills = append(collectSkills, s)
	}
	return []TaskProposal{
		{
			Title:          fmt.Sprintf("جمع‌آوری داده‌های %s", subject),
			Description:    fmt.Sprintf("گردآوری آخرین آمار و مواد لازم برای: %s.", intent.Goal),
			Topic:          primary,
			RequiredSkills: collectSkills,
			Dependencies:   []int{},
			ExpectedOutput: fmt.Sprintf("داده‌های راستی‌آزمایی‌شده و یادداشت‌هایی دربارهٔ %s", subject),
		},
		{
			Title:          fmt.Sprintf("نوشتن پیش‌نویس خروجی"),
			Description:    fmt.Sprintf("نوشتن پیش‌نویس روشنی برای برآورده‌کردن این هدف: %s. از داده‌های جمع‌آوری‌شده استفاده کنید.", intent.Goal),
			Topic:          "",
			RequiredSkills: []string{"writing"},
			Dependencies:   []int{0},
			ExpectedOutput: "پیش‌نویس کاملِ منطبق با درخواست مدیر",
		},
		{
			Title:          "Review and finalize",
			Description:    "بررسی صحت، صیقل نگارش و آماده‌سازی نسخهٔ نهایی برای انتشار.",
			Topic:          "",
			RequiredSkills: []string{"review"},
			Dependencies:   []int{1},
			ExpectedOutput: "سند نهایی تأییدشده و آمادهٔ اشتراک",
		},
	}
}

func generalTasks(intent IntentResult) []TaskProposal {
	primary := firstNonEmpty(intent.Topics...)
	skills := []string{"execution"}
	if s := skillsForTopic(primary); s != "" {
		skills = append(skills, s)
	}
	return []TaskProposal{
		{
			Title:          intent.Title,
			Description:    fmt.Sprintf("انجام کامل این درخواست: %s.", intent.Goal),
			Topic:          primary,
			RequiredSkills: skills,
			Dependencies:   []int{},
			ExpectedOutput: "درخواست مدیر برآورده شد",
		},
	}
}

func validatePlan(tasks []TaskProposal) error {
	if len(tasks) == 0 {
		return fmt.Errorf("%w: plan has no tasks", ErrInvalidLLMOutput)
	}
	for i := range tasks {
		tp := &tasks[i]
		if strings.TrimSpace(tp.Title) == "" {
			return fmt.Errorf("%w: task %d has no title", ErrInvalidLLMOutput, i)
		}
		for _, dep := range tp.Dependencies {
			if dep < 0 || dep >= len(tasks) || dep == i {
				return fmt.Errorf("%w: task %d has invalid dependency %d", ErrInvalidLLMOutput, i, dep)
			}
		}
	}
	return nil
}

const planSchema = `{"title":"workflow title","rationale":"why this decomposition works","tasks":[{"title":"...","description":"...","topic":"primary topic or empty string","required_skills":["..."],"dependencies":[0],"expected_output":"..."}]}`

type llmPlanningAgent struct {
	provider LLMProvider
}

func (l *llmPlanningAgent) ProposePlan(ctx context.Context, intent IntentResult, org *OrgContext) (PlanResult, error) {
	instruction := fmt.Sprintf(
		"Decompose this manager request into 1-4 concrete tasks with dependencies expressed as zero-based indexes.\nRequest: %q\nKind: %s\nTopics: %v\nTeam members: %s",
		intent.Goal, intent.Kind, intent.Topics, describePeople(org),
	)
	var result PlanResult
	if err := l.provider.GenerateStructured(ctx, StructuredRequest{
		Instruction: instruction,
		SchemaHint:  planSchema,
	}, &result); err != nil {
		return PlanResult{}, err
	}
	result.Title = firstNonEmpty(result.Title, intent.Title)
	if err := validatePlan(result.Tasks); err != nil {
		return PlanResult{}, err
	}
	return result, nil
}

func describePeople(org *OrgContext) string {
	names := make([]string, 0, len(org.People))
	for _, p := range org.People {
		names = append(names, fmt.Sprintf("%s (skills: %s)", p.Name, strings.Join(p.Skills, ", ")))
	}
	return strings.Join(names, "; ")
}
