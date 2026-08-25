package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/apa/backend/internal/domain/approval"
)

type Approvals struct {
	pool *pgxpool.Pool
}

func NewApprovals(pool *pgxpool.Pool) *Approvals {
	return &Approvals{pool: pool}
}

func (r *Approvals) Create(ctx context.Context, a *approval.Approval) error {
	err := r.pool.QueryRow(ctx,
		`INSERT INTO approvals (org_id, workflow_id, type, status, payload)
		 VALUES ($1,$2,$3,$4,$5::jsonb)
		 RETURNING id, created_at`,
		a.OrgID, a.WorkflowID, string(a.Type), string(a.Status), string(a.Payload),
	).Scan(&a.ID, &a.CreatedAt)
	return mapErr(err)
}

func (r *Approvals) LatestForWorkflow(ctx context.Context, workflowID uuid.UUID, typ approval.Type) (*approval.Approval, error) {
	var a approval.Approval
	var typeStr, statusStr string
	err := r.pool.QueryRow(ctx,
		`SELECT id, org_id, workflow_id, type, status, payload, decided_by, decided_at, created_at
		 FROM approvals WHERE workflow_id = $1 AND type = $2
		 ORDER BY created_at DESC LIMIT 1`, workflowID, string(typ),
	).Scan(&a.ID, &a.OrgID, &a.WorkflowID, &typeStr, &statusStr, &a.Payload, &a.DecidedBy, &a.DecidedAt, &a.CreatedAt)
	if err != nil {
		return nil, mapErr(err)
	}
	a.Type = approval.Type(typeStr)
	a.Status = approval.Status(statusStr)
	return &a, nil
}

func (r *Approvals) Decide(ctx context.Context, id uuid.UUID, status approval.Status, decidedBy uuid.UUID, at time.Time) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE approvals SET status = $2, decided_by = $3, decided_at = $4
		 WHERE id = $1 AND status = 'pending'`,
		id, string(status), decidedBy, at)
	return expectAffected(tag, err, "approval already decided")
}
