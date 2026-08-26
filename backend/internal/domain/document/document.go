package document

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/apa/backend/internal/domain"
)

type Status string

const (
	StatusGenerating Status = "generating"
	StatusReady      Status = "ready"
	StatusFailed     Status = "failed"
)

type Document struct {
	ID          uuid.UUID  `json:"id"`
	OrgID       uuid.UUID  `json:"orgId"`
	TaskID      *uuid.UUID `json:"taskId,omitempty"`
	WorkflowID  *uuid.UUID `json:"workflowId,omitempty"`
	AuthorID    uuid.UUID  `json:"authorId"`
	AuthorName  string     `json:"authorName,omitempty"`
	Title       string     `json:"title"`
	Body        string     `json:"body"`
	SourceNotes string     `json:"sourceNotes,omitempty"`
	Status      Status     `json:"status"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
}

func ValidateSource(content string) error {
	if len(strings.TrimSpace(content)) < 10 {
		return domain.Invalid("content", "توضیح دهید چه کاری انجام شده (حداقل ۱۰ نویسه)")
	}
	return nil
}

func MarkReady(d *Document, title, body string) {
	d.Title = strings.TrimSpace(title)
	if d.Title == "" {
		d.Title = "سند بدون عنوان"
	}
	d.Body = body
	d.Status = StatusReady
	d.UpdatedAt = time.Now().UTC()
}

func MarkFailed(d *Document, reason string) {
	d.Status = StatusFailed
	d.Body = fmt.Sprintf("ساخت سند ناموفق بود: %s", reason)
	d.UpdatedAt = time.Now().UTC()
}
