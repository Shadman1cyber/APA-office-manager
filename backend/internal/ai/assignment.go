package ai

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"

	"github.com/apa/backend/internal/domain/knowledge"
)

const (
	WeightOwnership    = 0.7
	WeightSkills       = 0.3
	MinAssignmentScore = 0.25
	MaxParallelScoring = 8
)

type AssignmentAgent interface {
	ProposeAssignments(ctx context.Context, tasks []TaskProposal, org *OrgContext) (map[int]AssignmentProposal, error)
}

func NewAssignmentAgent(p LLMProvider) AssignmentAgent {
	if _, ok := p.(*MockProvider); ok {
		return &mockAssignmentAgent{}
	}
	return &llmAssignmentAgent{provider: p}
}

type mockAssignmentAgent struct{}

func (m *mockAssignmentAgent) ProposeAssignments(ctx context.Context, tasks []TaskProposal, org *OrgContext) (map[int]AssignmentProposal, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	results := make([]AssignmentProposal, len(tasks))
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(MaxParallelScoring)

	for i := range tasks {
		i, tp := i, tasks[i]
		g.Go(func() error {
			results[i] = m.proposeForTask(ctx, tp, org)
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}

	out := make(map[int]AssignmentProposal, len(tasks))
	for i := range results {
		out[i] = results[i]
	}
	return out, nil
}

func (m *mockAssignmentAgent) proposeForTask(ctx context.Context, tp TaskProposal, org *OrgContext) AssignmentProposal {
	type scored struct {
		person   PersonInfo
		score    float64
		evidence []string
	}
	var candidates []scored
	for _, p := range org.People {
		score, evidence := ScoreCandidate(tp, p, org.Facts)
		candidates = append(candidates, scored{person: p, score: score, evidence: evidence})
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].score != candidates[j].score {
			return candidates[i].score > candidates[j].score
		}
		return candidates[i].person.Name < candidates[j].person.Name
	})

	if len(candidates) == 0 || candidates[0].score < MinAssignmentScore {
		return AssignmentProposal{
			Evidence:                  []string{"هنوز هم‌تیمی مناسبی برای این وظیفه مشخص نیست؛ منتظر راهنمایی هستم."},
			Confidence:                0,
			RequiresHumanConfirmation: true,
		}
	}

	best := candidates[0]
	id := best.person.ID.String()
	return AssignmentProposal{
		CandidateUserID:           &id,
		CandidateName:             best.person.Name,
		Evidence:                  best.evidence,
		Confidence:                round2(best.score),
		RequiresHumanConfirmation: true,
	}
}

func ScoreCandidate(tp TaskProposal, person PersonInfo, facts []knowledge.Fact) (float64, []string) {
	var evidence []string
	ownership := 0.0
	for _, f := range facts {
		if f.Kind == knowledge.KindTopicOwner &&
			f.PersonID == person.ID &&
			strings.EqualFold(f.Subject, tp.Topic) &&
			f.Confidence > ownership {
			ownership = f.Confidence
			sourceFa := map[string]string{"seeded": "اولیه", "learned": "یادگرفته‌شده"}[string(f.Source)]
			evidence = append(evidence, fmt.Sprintf(
				"%s مسئول «%s» است (اطمینان %.2f، %d بار مشاهده، منبع: %s)",
				person.Name, f.Subject, f.Confidence, f.EvidenceCount, sourceFa,
			))
		}
	}

	var matches []string
	for _, skill := range tp.RequiredSkills {
		for _, s := range person.Skills {
			if strings.EqualFold(skill, s) {
				matches = append(matches, skill)
				break
			}
		}
	}
	skillRatio := 0.0
	if len(tp.RequiredSkills) > 0 {
		skillRatio = float64(len(matches)) / float64(len(tp.RequiredSkills))
	}
	for _, skill := range matches {
		evidence = append(evidence, fmt.Sprintf("تطابق مهارت: %s", skill))
	}

	score := WeightOwnership*ownership + WeightSkills*skillRatio
	return score, evidence
}

func round2(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}

const assignmentsSchema = `{"assignments":[{"task_index":0,"candidate_user_id":"uuid or null","candidate_name":"","evidence":["..."],"confidence":0.42,"requires_human_confirmation":true}]}`

type llmAssignmentAgent struct {
	provider LLMProvider
}

type llmAssignmentWire struct {
	TaskIndex            int      `json:"task_index"`
	CandidateUserID      *string  `json:"candidate_user_id"`
	CandidateName        string   `json:"candidate_name"`
	Evidence             []string `json:"evidence"`
	Confidence           float64  `json:"confidence"`
	RequiresHumanConfirm bool     `json:"requires_human_confirmation"`
}

type llmAssignmentsWire struct {
	Assignments []llmAssignmentWire `json:"assignments"`
}

func (l *llmAssignmentAgent) ProposeAssignments(ctx context.Context, tasks []TaskProposal, org *OrgContext) (map[int]AssignmentProposal, error) {
	validIDs := map[string]bool{}
	for _, p := range org.People {
		validIDs[p.ID.String()] = true
	}
	instruction := fmt.Sprintf(
		"For each task choose the single best owner among the teammates, or nobody if unsure.\nTasks:\n%s\nTeammates:\n%s\nKnowledge:\n%s",
		describeTasks(tasks), describePeopleIDs(org), describeFacts(org),
	)
	var wire llmAssignmentsWire
	if err := l.provider.GenerateStructured(ctx, StructuredRequest{
		Instruction: instruction,
		SchemaHint:  assignmentsSchema,
	}, &wire); err != nil {
		return nil, err
	}

	results := make([]AssignmentProposal, len(wire.Assignments))
	used := make([]bool, len(tasks))
	for _, a := range wire.Assignments {
		if a.TaskIndex < 0 || a.TaskIndex >= len(tasks) || used[a.TaskIndex] {
			continue
		}
		proposal := AssignmentProposal{
			TaskID:                    fmt.Sprintf("%d", a.TaskIndex),
			Evidence:                  a.Evidence,
			Confidence:                clamp01(a.Confidence),
			RequiresHumanConfirmation: true,
		}
		if a.CandidateUserID != nil && validIDs[*a.CandidateUserID] {
			proposal.CandidateUserID = a.CandidateUserID
			proposal.CandidateName = a.CandidateName
			if len(proposal.Evidence) == 0 {
				proposal.Evidence = []string{"انتخاب مدل بر اساس زمینهٔ موجود."}
			}
		} else if len(proposal.Evidence) == 0 {
			proposal.Evidence = []string{"هیچ نامزد مطمئنی شناسایی نشد."}
		}
		results[a.TaskIndex] = proposal
		used[a.TaskIndex] = true
	}

	out := make(map[int]AssignmentProposal)
	for i := range used {
		if used[i] {
			out[i] = results[i]
		}
	}
	return out, nil
}

func describePeopleIDs(org *OrgContext) string {
	out := ""
	for _, p := range org.People {
		out += fmt.Sprintf("- %s id=%s skills=%v\n", p.Name, p.ID, p.Skills)
	}
	return out
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

var _ = uuid.Nil
