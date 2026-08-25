package application

import (
	"time"

	"github.com/google/uuid"
)

type ChatRole string

const (
	ChatRoleUser      ChatRole = "user"
	ChatRoleAssistant ChatRole = "assistant"
)

type ChatMessage struct {
	ID         int64      `json:"id"`
	OrgID      uuid.UUID  `json:"orgId"`
	UserID     uuid.UUID  `json:"userId"`
	Role       ChatRole   `json:"role"`
	Text       string     `json:"text"`
	Action     string     `json:"action,omitempty"`
	WorkflowID *uuid.UUID `json:"workflowId,omitempty"`
	QuestionID *uuid.UUID `json:"questionId,omitempty"`
	CreatedAt  time.Time  `json:"createdAt"`
}

type ChatDaySummary struct {
	Day     string `json:"day"`
	Count   int    `json:"count"`
	Preview string `json:"preview"`
}
