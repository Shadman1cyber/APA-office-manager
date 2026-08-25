package question

import (
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/apa/backend/internal/domain"
)

type Status string

const (
	StatusOpen     Status = "open"
	StatusAnswered Status = "answered"
)

type Question struct {
	ID            uuid.UUID  `json:"id"`
	OrgID         uuid.UUID  `json:"orgId"`
	WorkflowID    uuid.UUID  `json:"workflowId"`
	TaskIndex     int        `json:"-"`
	RelatedTaskID *uuid.UUID `json:"relatedTaskId,omitempty"`
	Topic         string     `json:"topic"`
	Text          string     `json:"question"`
	Reason        string     `json:"reason"`
	Required      bool       `json:"required"`
	Status        Status     `json:"status"`
	Answer        string     `json:"answer,omitempty"`
	AnsweredBy    *uuid.UUID `json:"answeredBy,omitempty"`
	CreatedAt     time.Time  `json:"createdAt"`
	AnsweredAt    *time.Time `json:"answeredAt,omitempty"`
}

func (q *Question) ApplyAnswer(text string, by uuid.UUID, now time.Time) error {
	if q.Status == StatusAnswered {
		return fmt.Errorf("%w: این سؤال قبلاً پاسخ داده شده است", domain.ErrInvalidState)
	}
	if text == "" {
		return domain.Invalid("answer", "متن پاسخ الزامی است")
	}
	q.Answer = text
	q.AnsweredBy = &by
	q.Status = StatusAnswered
	q.AnsweredAt = &now
	return nil
}
