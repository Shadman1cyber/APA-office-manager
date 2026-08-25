package approval

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/apa/backend/internal/domain"
)

type Type string

const (
	TypePlan Type = "plan"
)

type Status string

const (
	StatusPending  Status = "pending"
	StatusApproved Status = "approved"
	StatusRejected Status = "rejected"
)

type Approval struct {
	ID         uuid.UUID       `json:"id"`
	OrgID      uuid.UUID       `json:"orgId"`
	WorkflowID uuid.UUID       `json:"workflowId"`
	Type       Type            `json:"type"`
	Status     Status          `json:"status"`
	Payload    json.RawMessage `json:"payload"`
	DecidedBy  *uuid.UUID      `json:"decidedBy,omitempty"`
	DecidedAt  *time.Time      `json:"decidedAt,omitempty"`
	CreatedAt  time.Time       `json:"createdAt"`
}

func NewPlan(orgID, workflowID uuid.UUID, payload json.RawMessage) (*Approval, error) {
	if len(payload) == 0 {
		payload = json.RawMessage(`{}`)
	}
	if !json.Valid(payload) {
		return nil, fmt.Errorf("%w: approval payload is not valid JSON", domain.ErrInvalidState)
	}
	return &Approval{
		OrgID:      orgID,
		WorkflowID: workflowID,
		Type:       TypePlan,
		Status:     StatusPending,
		Payload:    payload,
		CreatedAt:  time.Now().UTC(),
	}, nil
}

func (a *Approval) Decide(status Status, by uuid.UUID, now time.Time) error {
	if a.Status != StatusPending {
		return fmt.Errorf("%w: approval already decided (%s)", domain.ErrInvalidState, a.Status)
	}
	if status != StatusApproved && status != StatusRejected {
		return fmt.Errorf("%w: invalid approval decision %q", domain.ErrInvalidState, status)
	}
	a.Status = status
	a.DecidedBy = &by
	a.DecidedAt = &now
	return nil
}
