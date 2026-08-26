package task

import (
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/apa/backend/internal/domain"
)

type Status string

const (
	StatusProposed   Status = "proposed"
	StatusPending    Status = "pending"
	StatusAssigned   Status = "assigned"
	StatusInProgress Status = "in_progress"
	StatusCompleted  Status = "completed"
	StatusVerified   Status = "verified"
	StatusBlocked    Status = "blocked"
)

func ParseStatus(raw string) (Status, error) {
	s := Status(raw)
	switch s {
	case StatusProposed, StatusPending, StatusAssigned, StatusInProgress, StatusCompleted, StatusVerified, StatusBlocked:
		return s, nil
	default:
		return "", fmt.Errorf("%w: unknown task status %q", domain.ErrInvalidState, raw)
	}
}

var allowedTransitions = map[Status][]Status{
	StatusProposed:   {StatusPending, StatusAssigned},
	StatusPending:    {StatusAssigned},
	StatusAssigned:   {StatusInProgress},
	StatusInProgress: {StatusCompleted, StatusBlocked},
	StatusCompleted:  {StatusVerified, StatusBlocked},
	StatusVerified:   {},
	StatusBlocked:    {StatusInProgress, StatusAssigned},
}

func CanTransition(from, to Status) bool {
	for _, next := range allowedTransitions[from] {
		if next == to {
			return true
		}
	}
	return false
}

type Proposal struct {
	CandidateUserID           *uuid.UUID `json:"candidateUserId,omitempty"`
	CandidateName             string     `json:"candidateName,omitempty"`
	Evidence                  []string   `json:"evidence"`
	Confidence                float64    `json:"confidence"`
	RequiresHumanConfirmation bool       `json:"requiresHumanConfirmation"`
}

func (p *Proposal) Validate() error {
	if p.Confidence < 0 || p.Confidence > 1 {
		return fmt.Errorf("%w: assignment confidence %.2f outside [0,1]", domain.ErrInvalidState, p.Confidence)
	}
	if p.CandidateUserID != nil && p.Confidence > 0 && len(p.Evidence) == 0 {
		return fmt.Errorf("%w: assignment proposal without evidence", domain.ErrInvalidState)
	}
	return nil
}

type Task struct {
	ID             uuid.UUID   `json:"id"`
	OrgID          uuid.UUID   `json:"orgId"`
	WorkflowID     uuid.UUID   `json:"workflowId"`
	Position       int         `json:"position"`
	Title          string      `json:"title"`
	Description    string      `json:"description"`
	Topic          string      `json:"topic"`
	RequiredSkills []string    `json:"requiredSkills"`
	DependsOn      []uuid.UUID `json:"dependsOn"`
	ExpectedOutput string      `json:"expectedOutput"`
	Status         Status      `json:"status"`
	Deadline       *time.Time  `json:"deadline,omitempty"`
	AssignedTo     *uuid.UUID  `json:"assignedTo,omitempty"`
	AssigneeName   string      `json:"assigneeName,omitempty"`
	Proposal       *Proposal   `json:"proposal,omitempty"`
	CompletedNotes string      `json:"completedNotes,omitempty"`
	VerifiedAt     *time.Time  `json:"verifiedAt,omitempty"`
	CreatedAt      time.Time   `json:"createdAt"`
	UpdatedAt      time.Time   `json:"updatedAt"`
}

func (t *Task) TransitionTo(to Status) error {
	if !CanTransition(t.Status, to) {
		return fmt.Errorf("%w: task cannot move from %s to %s", domain.ErrInvalidState, t.Status, to)
	}
	t.Status = to
	t.UpdatedAt = time.Now().UTC()
	return nil
}

func (t *Task) DependenciesSatisfied(statusOf func(uuid.UUID) (Status, error)) error {
	for _, dep := range t.DependsOn {
		st, err := statusOf(dep)
		if err != nil {
			return err
		}
		if st != StatusVerified {
			return fmt.Errorf("%w: وظیفهٔ وابسته هنوز به پایان نرسیده است (%s، وضعیت: %s)", domain.ErrInvalidState, dep, st)
		}
	}
	return nil
}
