package ai

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/google/uuid"

	"github.com/apa/backend/internal/domain/knowledge"
	"github.com/apa/backend/internal/domain/user"
)

type PersonInfo struct {
	ID     uuid.UUID `json:"id"`
	Name   string    `json:"name"`
	Skills []string  `json:"skills"`
}

type OrgContext struct {
	People []PersonInfo     `json:"people"`
	Facts  []knowledge.Fact `json:"facts"`
}

func NewOrgContext(users []user.User, facts []knowledge.Fact) *OrgContext {
	people := make([]PersonInfo, 0, len(users))
	for _, u := range users {
		people = append(people, PersonInfo{ID: u.ID, Name: u.Name, Skills: u.Skills})
	}
	return &OrgContext{People: people, Facts: facts}
}

func (o *OrgContext) FactFor(topic string) *knowledge.Fact {
	var best *knowledge.Fact
	for i := range o.Facts {
		f := &o.Facts[i]
		if f.Kind == knowledge.KindTopicOwner && strings.EqualFold(f.Subject, topic) {
			if best == nil || f.Confidence > best.Confidence {
				best = f
			}
		}
	}
	return best
}

func (o *OrgContext) PersonByID(id uuid.UUID) (PersonInfo, bool) {
	for _, p := range o.People {
		if p.ID == id {
			return p, true
		}
	}
	return PersonInfo{}, false
}

type OrgDataReader interface {
	ListUsers(ctx context.Context, orgID uuid.UUID) ([]user.User, error)
	FindFacts(ctx context.Context, orgID uuid.UUID, kind knowledge.FactKind, subjects []string) ([]knowledge.Fact, error)
}

type ContextAgent struct {
	reader OrgDataReader
}

func NewContextAgent(reader OrgDataReader) *ContextAgent {
	return &ContextAgent{reader: reader}
}

func (c *ContextAgent) Gather(ctx context.Context, orgID uuid.UUID) (*OrgContext, error) {
	users, err := c.reader.ListUsers(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("gather people: %w", err)
	}
	facts, err := c.reader.FindFacts(ctx, orgID, knowledge.KindTopicOwner, nil)
	if err != nil {
		return nil, fmt.Errorf("gather knowledge: %w", err)
	}
	return NewOrgContext(users, facts), nil
}

var topicKeywords = map[string]string{
	"cybersecurity":      "cybersecurity",
	"security awareness": "cybersecurity",
	"phishing":           "phishing awareness",
	"incident statistic": "incident statistics",
	"incident":           "incident statistics",
	"statistic":          "incident statistics",
	"budget":             "budgeting",
	"marketing":          "marketing",
	"onboarding":         "employee onboarding",
	"compliance":         "compliance",
	"audit":              "compliance",
	"customer feedback":  "customer feedback",
	"sales":              "sales reporting",
}

var sortedTopicKeywords = func() []string {
	keys := make([]string, 0, len(topicKeywords))
	for k := range topicKeywords {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return len(keys[i]) > len(keys[j])
	})
	return keys
}()

func DetectTopics(text string) []string {
	lower := strings.ToLower(text)
	var topics []string
	seen := map[string]bool{}
	for _, kw := range sortedTopicKeywords {
		if strings.Contains(lower, kw) {
			topic := topicKeywords[kw]
			if !seen[topic] {
				seen[topic] = true
				topics = append(topics, topic)
			}
		}
	}
	return topics
}

var smallTalkPatterns = []string{
	"hi", "hello", "hey", "yo", "hiya", "thanks", "thank you", "thx",
	"ok", "okay", "great", "nice", "cool", "good morning", "good afternoon",
	"good evening", "how are you", "lol", "test", "bye",
}

var taskVerbs = []string{
	"prepare", "make", "write", "create", "organize", "organise", "send",
	"review", "plan", "build", "draft", "collect", "schedule", "report",
	"summarize", "summarise", "update", "fix", "find", "check", "call",
	"meet", "email", "design", "research", "analyze", "analyse", "compile",
	"assign", "deliver", "set up", "clean up",
}

func containsTaskVerb(lower string) bool {
	for _, v := range taskVerbs {
		if strings.Contains(lower, v) {
			return true
		}
	}
	return false
}

func ClassifyIntent(text string) string {
	lower := strings.ToLower(strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(text), ".")))
	for _, p := range smallTalkPatterns {
		if lower == p || strings.HasPrefix(lower, p+" ") || strings.HasPrefix(lower, p+",") {
			return IntentKindSmallTalk
		}
	}
	if len([]rune(lower)) < 18 && !containsTaskVerb(lower) {
		return IntentKindSmallTalk
	}
	reportWords := []string{"report", "summary", "presentation", "brief"}
	for _, w := range reportWords {
		if strings.Contains(lower, w) {
			return IntentKindReport
		}
	}
	return IntentKindGeneral
}

func UpperFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
