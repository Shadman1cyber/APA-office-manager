package repository

import (
	"context"
	"strconv"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/apa/backend/internal/domain/question"
)

type Questions struct {
	pool *pgxpool.Pool
}

func NewQuestions(pool *pgxpool.Pool) *Questions {
	return &Questions{pool: pool}
}

const questionColumns = `id, org_id, workflow_id, task_index, related_task_id, topic, question,
	reason, required, status, answer, answered_by, created_at, answered_at`

func scanQuestion(row rowScanner) (*question.Question, error) {
	var q question.Question
	var status string
	if err := row.Scan(
		&q.ID, &q.OrgID, &q.WorkflowID, &q.TaskIndex, &q.RelatedTaskID, &q.Topic,
		&q.Text, &q.Reason, &q.Required, &status, &q.Answer, &q.AnsweredBy,
		&q.CreatedAt, &q.AnsweredAt,
	); err != nil {
		return nil, mapErr(err)
	}
	q.Status = question.Status(status)
	return &q, nil
}

func (r *Questions) Create(ctx context.Context, q *question.Question) error {
	err := r.pool.QueryRow(ctx,
		`INSERT INTO questions (org_id, workflow_id, task_index, related_task_id, topic, question, reason, required, status)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		 RETURNING id, created_at`,
		q.OrgID, q.WorkflowID, q.TaskIndex, q.RelatedTaskID, q.Topic,
		q.Text, q.Reason, q.Required, string(q.Status),
	).Scan(&q.ID, &q.CreatedAt)
	return mapErr(err)
}

func (r *Questions) Get(ctx context.Context, orgID, id uuid.UUID) (*question.Question, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, org_id, workflow_id, task_index, related_task_id, topic, question,
		        reason, required, status, answer, answered_by, created_at, answered_at
		 FROM questions WHERE id = $1 AND org_id = $2`, id, orgID)
	return scanQuestion(row)
}

func (r *Questions) ListByWorkflow(ctx context.Context, workflowID uuid.UUID) ([]*question.Question, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, org_id, workflow_id, task_index, related_task_id, topic, question,
		        reason, required, status, answer, answered_by, created_at, answered_at
		 FROM questions WHERE workflow_id = $1 ORDER BY created_at`, workflowID)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	return collectQuestions(rows)
}

func (r *Questions) ListByOrg(ctx context.Context, orgID uuid.UUID, status string, workflowID *uuid.UUID) ([]*question.Question, error) {
	query := `SELECT id, org_id, workflow_id, task_index, related_task_id, topic, question,
	                 reason, required, status, answer, answered_by, created_at, answered_at
	          FROM questions WHERE org_id = $1`
	args := []any{orgID}
	argN := 2
	if status != "" {
		query += " AND status = $" + strconv.Itoa(argN)
		args = append(args, status)
		argN++
	}
	if workflowID != nil {
		query += " AND workflow_id = $" + strconv.Itoa(argN)
		args = append(args, *workflowID)
		argN++
	}
	query += " ORDER BY created_at DESC LIMIT 200"

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	return collectQuestions(rows)
}

func (r *Questions) ListOpenRequired(ctx context.Context, workflowID uuid.UUID) ([]*question.Question, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, org_id, workflow_id, task_index, related_task_id, topic, question,
		        reason, required, status, answer, answered_by, created_at, answered_at
		 FROM questions WHERE workflow_id = $1 AND status = 'open' AND required = true
		 ORDER BY created_at`, workflowID)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	return collectQuestions(rows)
}

func (r *Questions) PersistAnswer(ctx context.Context, q *question.Question) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE questions SET status = 'answered', answer = $3, answered_by = $4, answered_at = now()
		 WHERE id = $1 AND org_id = $2 AND status = 'open'`,
		q.ID, q.OrgID, q.Answer, q.AnsweredBy)
	return expectAffected(tag, err, "question already answered or missing")
}

func collectQuestions(rows pgx.Rows) ([]*question.Question, error) {
	result := []*question.Question{}
	for rows.Next() {
		q, err := scanQuestion(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, q)
	}
	return result, rows.Err()
}
