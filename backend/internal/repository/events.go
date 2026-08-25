package repository

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/apa/backend/internal/application"
)

type Events struct {
	pool *pgxpool.Pool
}

func NewEvents(pool *pgxpool.Pool) *Events {
	return &Events{pool: pool}
}

func (r *Events) Append(ctx context.Context, e *application.Event) error {
	payload, err := json.Marshal(e.Payload)
	if err != nil {
		return err
	}
	return r.pool.QueryRow(ctx,
		`INSERT INTO events (org_id, type, entity_type, entity_id, actor_id, payload)
		 VALUES ($1,$2,$3,$4,$5,$6::jsonb)
		 RETURNING id, created_at`,
		e.OrgID, string(e.Type), e.EntityType, e.EntityID, e.ActorID, string(payload),
	).Scan(&e.ID, &e.Timestamp)
}

func (r *Events) List(ctx context.Context, orgID uuid.UUID, entityType string, entityID string, limit int) ([]*application.Event, error) {
	query := `SELECT id, org_id, type, entity_type, entity_id, actor_id, payload, created_at
	          FROM events WHERE org_id = $1`
	args := []any{orgID}
	argN := 2
	if entityType != "" {
		query += " AND entity_type = $" + strconv.Itoa(argN)
		args = append(args, entityType)
		argN++
	}
	if entityID != "" {
		query += " AND entity_id = $" + strconv.Itoa(argN)
		args = append(args, entityID)
		argN++
	}
	query += " ORDER BY created_at DESC LIMIT $" + strconv.Itoa(argN)
	args = append(args, limit)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()

	result := []*application.Event{}
	for rows.Next() {
		var e application.Event
		var typeStr string
		var payloadBytes []byte
		if err := rows.Scan(&e.ID, &e.OrgID, &typeStr, &e.EntityType, &e.EntityID, &e.ActorID, &payloadBytes, &e.Timestamp); err != nil {
			return nil, mapErr(err)
		}
		e.Type = application.EventType(typeStr)
		e.Payload = map[string]any{}
		if len(payloadBytes) > 0 {
			if err := json.Unmarshal(payloadBytes, &e.Payload); err != nil {
				return nil, err
			}
		}
		result = append(result, &e)
	}
	return result, rows.Err()
}
