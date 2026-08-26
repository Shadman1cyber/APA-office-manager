package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/apa/backend/internal/domain/task"
)

type Tasks struct {
	pool *pgxpool.Pool
}

func NewTasks(pool *pgxpool.Pool) *Tasks {
	return &Tasks{pool: pool}
}

func (r *Tasks) CreateBatch(ctx context.Context, list []*task.Task) error {
	batch := &pgx.Batch{}
	for _, t := range list {
		batch.Queue(
			`INSERT INTO tasks (org_id, workflow_id, position, title, description, topic,
			                    required_skills, depends_on, expected_output, status, deadline, assigned_to)
			 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
			 RETURNING id, created_at, updated_at`,
			t.OrgID, t.WorkflowID, t.Position, t.Title, t.Description, t.Topic,
			textArray(t.RequiredSkills), uuidArray(t.DependsOn), nullIfEmptyStr(t.ExpectedOutput),
			string(t.Status), nullableTime(t.Deadline), t.AssignedTo,
		)
	}
	br := r.pool.SendBatch(ctx, batch)
	defer br.Close()

	for _, t := range list {
		if err := br.QueryRow().Scan(&t.ID, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return mapErr(err)
		}
	}
	return nil
}

func (r *Tasks) SetDependencies(ctx context.Context, deps map[uuid.UUID][]uuid.UUID) error {
	if len(deps) == 0 {
		return nil
	}
	batch := &pgx.Batch{}
	for id, depList := range deps {
		batch.Queue(`UPDATE tasks SET depends_on = $2, updated_at = now() WHERE id = $1`, id, depList)
	}
	br := r.pool.SendBatch(ctx, batch)
	defer br.Close()
	for range deps {
		if _, err := br.Exec(); err != nil {
			return mapErr(err)
		}
	}
	return nil
}

const taskColumns = `t.id, t.org_id, t.workflow_id, t.position, t.title, t.description, t.topic,
	t.required_skills, t.depends_on, t.expected_output, t.status, t.assigned_to, u.name,
	t.deadline, t.proposal, t.completed_notes, t.verified_at, t.created_at, t.updated_at`

const taskFrom = `FROM tasks t LEFT JOIN users u ON u.id = t.assigned_to`

type rowScanner interface {
	Scan(dest ...any) error
}

func scanTask(row rowScanner) (*task.Task, error) {
	var t task.Task
	var status string
	var proposalBytes []byte
	var assignedTo *uuid.UUID
	var assigneeName *string
	var deadline *time.Time
	if err := row.Scan(
		&t.ID, &t.OrgID, &t.WorkflowID, &t.Position, &t.Title, &t.Description, &t.Topic,
		&t.RequiredSkills, &t.DependsOn, &t.ExpectedOutput, &status, &assignedTo, &assigneeName,
		&deadline, &proposalBytes, &t.CompletedNotes, &t.VerifiedAt, &t.CreatedAt, &t.UpdatedAt,
	); err != nil {
		return nil, mapErr(err)
	}
	t.Status = task.Status(status)
	t.AssignedTo = assignedTo
	t.Deadline = deadline
	if assigneeName != nil {
		t.AssigneeName = *assigneeName
	}
	if len(proposalBytes) > 0 {
		p := &task.Proposal{}
		if err := json.Unmarshal(proposalBytes, p); err != nil {
			return nil, fmt.Errorf("decode task proposal: %w", err)
		}
		t.Proposal = p
	}
	return &t, nil
}

func collectTasks(rows pgx.Rows) ([]*task.Task, error) {
	result := []*task.Task{}
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, t)
	}
	return result, rows.Err()
}

func (r *Tasks) Get(ctx context.Context, orgID, id uuid.UUID) (*task.Task, error) {
	row := r.pool.QueryRow(ctx,
		fmt.Sprintf(`SELECT %s %s WHERE t.id = $1 AND t.org_id = $2`, taskColumns, taskFrom), id, orgID)
	return scanTask(row)
}

func (r *Tasks) ListByWorkflow(ctx context.Context, workflowID uuid.UUID) ([]*task.Task, error) {
	rows, err := r.pool.Query(ctx,
		fmt.Sprintf(`SELECT %s %s WHERE t.workflow_id = $1 ORDER BY t.position`, taskColumns, taskFrom), workflowID)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	return collectTasks(rows)
}

func (r *Tasks) ListByOrg(ctx context.Context, orgID uuid.UUID, assignedTo *uuid.UUID, status string, limit int) ([]*task.Task, error) {
	query := fmt.Sprintf(`SELECT %s %s WHERE t.org_id = $1`, taskColumns, taskFrom)
	args := []any{orgID}
	argN := 2
	if assignedTo != nil {
		query += fmt.Sprintf(" AND t.assigned_to = $%d", argN)
		args = append(args, *assignedTo)
		argN++
	}
	if status != "" {
		query += fmt.Sprintf(" AND t.status = $%d", argN)
		args = append(args, status)
		argN++
	}
	query += fmt.Sprintf(" ORDER BY t.created_at DESC LIMIT $%d", argN)
	args = append(args, limit)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	return collectTasks(rows)
}

func (r *Tasks) UserInvolvedInWorkflow(ctx context.Context, orgID, workflowID, userID uuid.UUID) (bool, error) {
	var n int
	err := r.pool.QueryRow(ctx,
		`SELECT count(*) FROM tasks
		 WHERE org_id = $1 AND workflow_id = $2 AND assigned_to = $3`,
		orgID, workflowID, userID).Scan(&n)
	return n > 0, mapErr(err)
}

func (r *Tasks) ListAvailable(ctx context.Context, orgID uuid.UUID, limit int) ([]*task.Task, error) {
	rows, err := r.pool.Query(ctx,
		fmt.Sprintf(`SELECT %s %s WHERE t.org_id = $1 AND t.status = 'pending' AND t.assigned_to IS NULL
		             ORDER BY t.created_at ASC LIMIT $2`, taskColumns, taskFrom),
		orgID, limit)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	return collectTasks(rows)
}

func (r *Tasks) UpdateStatusExpected(ctx context.Context, id uuid.UUID, from, to task.Status) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE tasks SET status = $3, updated_at = now() WHERE id = $1 AND status = $2`,
		id, string(from), string(to))
	return expectAffected(tag, err, "وضعیت وظیفه هم‌زمان تغییر کرده است؛ صفحه را تازه‌سازی کنید")
}

func (r *Tasks) Assign(ctx context.Context, id uuid.UUID, userID *uuid.UUID, from, to task.Status) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE tasks SET assigned_to = $2, status = $4, updated_at = now()
		 WHERE id = $1 AND status = $3`,
		id, userID, string(from), string(to))
	return expectAffected(tag, err, "وضعیت وظیفه هم‌زمان تغییر کرده است؛ صفحه را تازه‌سازی کنید")
}

func (r *Tasks) SaveProposal(ctx context.Context, id uuid.UUID, p *task.Proposal) error {
	var raw any
	if p != nil {
		data, err := json.Marshal(p)
		if err != nil {
			return fmt.Errorf("encode task proposal: %w", err)
		}
		raw = string(data)
	}
	tag, err := r.pool.Exec(ctx,
		`UPDATE tasks SET proposal = $2::jsonb, updated_at = now() WHERE id = $1`, id, raw)
	return expectAffected(tag, err, "وظیفه یافت نشد")
}

func (r *Tasks) SetCompletionNotes(ctx context.Context, id uuid.UUID, notes string) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE tasks SET completed_notes = $2, updated_at = now() WHERE id = $1`, id, notes)
	return expectAffected(tag, err, "وظیفه یافت نشد")
}

func (r *Tasks) SetVerified(ctx context.Context, id uuid.UUID, at time.Time) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE tasks SET status = 'verified', verified_at = $2, updated_at = now()
		 WHERE id = $1 AND status = 'completed'`, id, at)
	return expectAffected(tag, err, "وضعیت وظیفه هم‌زمان تغییر کرده است؛ صفحه را تازه‌سازی کنید")
}

func (r *Tasks) BlockTask(ctx context.Context, id uuid.UUID, from task.Status) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE tasks SET status = 'blocked', updated_at = now() WHERE id = $1 AND status = $2`,
		id, string(from))
	return expectAffected(tag, err, "وضعیت وظیفه هم‌زمان تغییر کرده است؛ صفحه را تازه‌سازی کنید")
}

func (r *Tasks) WorkflowProgress(ctx context.Context, workflowID uuid.UUID) (int, int, error) {
	var total, verified int
	err := r.pool.QueryRow(ctx,
		`SELECT count(*), count(*) FILTER (WHERE status = 'verified')
		 FROM tasks WHERE workflow_id = $1`, workflowID,
	).Scan(&total, &verified)
	return total, verified, mapErr(err)
}

func textArray(v []string) any {
	if v == nil {
		return []string{}
	}
	return v
}

func uuidArray(v []uuid.UUID) any {
	if v == nil {
		return []uuid.UUID{}
	}
	return v
}

func nullIfEmptyStr(s string) any {
	if s == "" {
		return ""
	}
	return s
}

func nullableTime(t *time.Time) any {
	if t == nil {
		return nil
	}
	return *t
}

func (r *Tasks) SetDeadline(ctx context.Context, orgID, id uuid.UUID, deadline *time.Time) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE tasks SET deadline = $3, updated_at = now() WHERE id = $1 AND org_id = $2`,
		id, orgID, deadline)
	return expectAffected(tag, err, "وظیفه یافت نشد")
}
