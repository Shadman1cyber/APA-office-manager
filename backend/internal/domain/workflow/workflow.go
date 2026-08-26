package workflow

import (
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/apa/backend/internal/domain"
)

type Status string

const (
	StatusProposed   Status = "proposed"
	StatusApproved   Status = "approved"
	StatusRejected   Status = "rejected"
	StatusInProgress Status = "in_progress"
	StatusCompleted  Status = "completed"
	StatusFailed     Status = "failed"
	StatusCancelled  Status = "cancelled"
)

func ParseStatus(raw string) (Status, error) {
	s := Status(raw)
	switch s {
	case StatusProposed, StatusApproved, StatusRejected, StatusInProgress, StatusCompleted, StatusFailed, StatusCancelled:
		return s, nil
	default:
		return "", fmt.Errorf("%w: unknown workflow status %q", domain.ErrInvalidState, raw)
	}
}

var allowedTransitions = map[Status][]Status{
	StatusProposed:   {StatusApproved, StatusRejected, StatusCancelled},
	StatusApproved:   {StatusInProgress, StatusFailed, StatusCancelled},
	StatusInProgress: {StatusCompleted, StatusFailed, StatusCancelled},
	StatusCompleted:  {},
	StatusRejected:   {},
	StatusFailed:     {},
	StatusCancelled:  {},
}

func CanTransition(from, to Status) bool {
	for _, next := range allowedTransitions[from] {
		if next == to {
			return true
		}
	}
	return false
}

type Workflow struct {
	ID        uuid.UUID `json:"id"`
	OrgID     uuid.UUID `json:"orgId"`
	CreatedBy uuid.UUID `json:"createdBy"`
	Title     string    `json:"title"`
	Intent    string    `json:"intentText"`
	Status    Status    `json:"status"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func New(orgID, createdBy uuid.UUID, title, intent string) (*Workflow, error) {
	if title == "" {
		return nil, fmt.Errorf("%w: عنوان گردش‌کار خالی است", domain.ErrInsufficientData)
	}
	if intent == "" {
		return nil, fmt.Errorf("%w: متن درخواست خالی است", domain.ErrInsufficientData)
	}
	return &Workflow{
		OrgID:     orgID,
		CreatedBy: createdBy,
		Title:     title,
		Intent:    intent,
		Status:    StatusProposed,
	}, nil
}

func (w *Workflow) TransitionTo(to Status) error {
	if !CanTransition(w.Status, to) {
		return fmt.Errorf("%w: workflow cannot move from %s to %s", domain.ErrInvalidState, w.Status, to)
	}
	w.Status = to
	w.UpdatedAt = time.Now().UTC()
	return nil
}
