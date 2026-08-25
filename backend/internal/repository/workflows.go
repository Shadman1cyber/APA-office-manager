package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/apa/backend/internal/domain/workflow"
)

type Workflows struct {
	pool *pgxpool.Pool
}

func NewWorkflows(pool *pgxpool.Pool) *Workflows {
	return &Workflows{pool: pool}
}

func (r *Workflows) Create(ctx context.Context, w *workflow.Workflow) error {
	err := r.pool.QueryRow(ctx,
		`INSERT INTO workflows (org_id, created_by, title, intent_text, status)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, created_at, updated_at`,
		w.OrgID, w.CreatedBy, w.Title, w.Intent, string(w.Status),
	).Scan(&w.ID, &w.CreatedAt, &w.UpdatedAt)
	return mapErr(err)
}

const workflowColumns = `id, org_id, created_by, title, intent_text, status, created_at, updated_at`

func (r *Workflows) Get(ctx context.Context, orgID, id uuid.UUID) (*workflow.Workflow, error) {
	var w workflow.Workflow
	var status string
	err := r.pool.QueryRow(ctx,
		`SELECT id, org_id, created_by, title, intent_text, status, created_at, updated_at
		 FROM workflows WHERE id = $1 AND org_id = $2`, id, orgID,
	).Scan(&w.ID, &w.OrgID, &w.CreatedBy, &w.Title, &w.Intent, &status, &w.CreatedAt, &w.UpdatedAt)
	if err != nil {
		return nil, mapErr(err)
	}
	w.Status = workflow.Status(status)
	return &w, nil
}

func (r *Workflows) List(ctx context.Context, orgID uuid.UUID, limit int) ([]*workflow.Workflow, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, org_id, created_by, title, intent_text, status, created_at, updated_at
		 FROM workflows WHERE org_id = $1 ORDER BY created_at DESC LIMIT $2`, orgID, limit)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()

	result := []*workflow.Workflow{}
	for rows.Next() {
		var w workflow.Workflow
		var status string
		if err := rows.Scan(&w.ID, &w.OrgID, &w.CreatedBy, &w.Title, &w.Intent, &status, &w.CreatedAt, &w.UpdatedAt); err != nil {
			return nil, mapErr(err)
		}
		w.Status = workflow.Status(status)
		result = append(result, &w)
	}
	return result, rows.Err()
}

func (r *Workflows) ListForUser(ctx context.Context, orgID, userID uuid.UUID) ([]*workflow.Workflow, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT DISTINCT w.id, w.org_id, w.created_by, w.title, w.intent_text, w.status, w.created_at, w.updated_at
		 FROM workflows w JOIN tasks t ON t.workflow_id = w.id
		 WHERE w.org_id = $1 AND t.assigned_to = $2
		 ORDER BY w.created_at DESC LIMIT 100`, orgID, userID)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()

	result := []*workflow.Workflow{}
	for rows.Next() {
		var w workflow.Workflow
		var status string
		if err := rows.Scan(&w.ID, &w.OrgID, &w.CreatedBy, &w.Title, &w.Intent, &status, &w.CreatedAt, &w.UpdatedAt); err != nil {
			return nil, mapErr(err)
		}
		w.Status = workflow.Status(status)
		result = append(result, &w)
	}
	return result, rows.Err()
}

func (r *Workflows) UpdateStatusExpected(ctx context.Context, orgID, id uuid.UUID, from, to workflow.Status) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE workflows SET status = $4, updated_at = now()
		 WHERE id = $1 AND org_id = $2 AND status = $3`,
		id, orgID, string(from), string(to))
	return expectAffected(tag, err, "workflow state changed concurrently")
}
