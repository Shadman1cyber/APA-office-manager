package application

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
)

type EventType string

const (
	EventWorkflowCreated    EventType = "WORKFLOW_CREATED"
	EventWorkflowApproved   EventType = "WORKFLOW_APPROVED"
	EventWorkflowRejected   EventType = "WORKFLOW_REJECTED"
	EventWorkflowCompleted  EventType = "WORKFLOW_COMPLETED"
	EventPlanGenerated      EventType = "PLAN_GENERATED"
	EventQuestionCreated    EventType = "QUESTION_CREATED"
	EventQuestionAnswered   EventType = "QUESTION_ANSWERED"
	EventAssignmentProposed EventType = "ASSIGNMENT_PROPOSED"
	EventAssignmentApproved EventType = "ASSIGNMENT_APPROVED"
	EventTaskAssigned       EventType = "TASK_ASSIGNED"
	EventTaskStarted        EventType = "TASK_STARTED"
	EventTaskCompleted      EventType = "TASK_COMPLETED"
	EventTaskVerified       EventType = "TASK_VERIFIED"
	EventKnowledgeLearned   EventType = "KNOWLEDGE_LEARNED"
)

type Event struct {
	ID         int64          `json:"id"`
	Type       EventType      `json:"type"`
	OrgID      uuid.UUID      `json:"orgId"`
	EntityType string         `json:"entityType"`
	EntityID   string         `json:"entityId"`
	ActorID    *uuid.UUID     `json:"actorId,omitempty"`
	Timestamp  time.Time      `json:"timestamp"`
	Payload    map[string]any `json:"payload"`
}

func (e *Event) Clone() *Event {
	copied := *e
	copied.Payload = make(map[string]any, len(e.Payload))
	for k, v := range e.Payload {
		copied.Payload[k] = v
	}
	return &copied
}

type EventStore interface {
	Append(ctx context.Context, e *Event) error
	List(ctx context.Context, orgID uuid.UUID, entityType string, entityID string, limit int) ([]*Event, error)
}

type JobEnqueuer interface {
	Enqueue(ctx context.Context, jobType string, payload []byte, runAfter time.Time) error
}

type Bus struct {
	store EventStore
	log   *slog.Logger

	mu   sync.Mutex
	subs []chan *Event
}

func NewBus(store EventStore, log *slog.Logger) *Bus {
	return &Bus{store: store, log: log}
}

func (b *Bus) Subscribe(buffer int) <-chan *Event {
	ch := make(chan *Event, buffer)
	b.mu.Lock()
	defer b.mu.Unlock()
	b.subs = append(b.subs, ch)
	return ch
}

func (b *Bus) Publish(ctx context.Context, e *Event) {
	e.Timestamp = time.Now().UTC()
	if e.Payload == nil {
		e.Payload = map[string]any{}
	}
	if err := b.store.Append(ctx, e); err != nil {
		b.log.ErrorContext(ctx, "persist event failed",
			slog.String("event_type", string(e.Type)),
			slog.Any("error", err),
		)
	}
	b.fanout(e.Clone())
}

func (b *Bus) fanout(e *Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, sub := range b.subs {
		select {
		case sub <- e:
		default:
		}
	}
}

func (b *Bus) ListRecent(ctx context.Context, orgID uuid.UUID, entityType, entityID string, limit int) ([]*Event, error) {
	return b.store.List(ctx, orgID, entityType, entityID, limit)
}
