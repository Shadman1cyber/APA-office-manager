package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Job struct {
	ID          int64
	Type        string
	Payload     []byte
	Attempts    int
	MaxAttempts int
}

type Jobs struct {
	pool *pgxpool.Pool
}

func NewJobs(pool *pgxpool.Pool) *Jobs {
	return &Jobs{pool: pool}
}

func (r *Jobs) Enqueue(ctx context.Context, jobType string, payload []byte, runAfter time.Time) error {
	if payload == nil {
		payload = []byte(`{}`)
	}
	_, err := r.pool.Exec(ctx,
		`INSERT INTO jobs (type, payload, run_after) VALUES ($1, $2::jsonb, $3)`,
		jobType, string(payload), runAfter)
	return mapErr(err)
}

const claimQuery = `
UPDATE jobs SET status = 'processing', attempts = attempts + 1, updated_at = now()
WHERE id IN (
	SELECT id FROM jobs
	WHERE status = 'pending' AND run_after <= now()
	ORDER BY id
	LIMIT $1
	FOR UPDATE SKIP LOCKED
)
RETURNING id, type, payload, attempts, max_attempts`

func (r *Jobs) Claim(ctx context.Context, limit int) ([]Job, error) {
	rows, err := r.pool.Query(ctx, claimQuery, limit)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()

	jobs := []Job{}
	for rows.Next() {
		var j Job
		if err := rows.Scan(&j.ID, &j.Type, &j.Payload, &j.Attempts, &j.MaxAttempts); err != nil {
			return nil, mapErr(err)
		}
		jobs = append(jobs, j)
	}
	return jobs, rows.Err()
}

func (r *Jobs) MarkDone(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE jobs SET status = 'done', updated_at = now() WHERE id = $1`, id)
	return mapErr(err)
}

func (r *Jobs) MarkFailed(ctx context.Context, id int64, lastErr string, retry bool, backoff time.Duration) error {
	if retry {
		tag, err := r.pool.Exec(ctx,
			`UPDATE jobs SET status = 'pending', last_error = $2, run_after = now() + make_interval(secs => $3), updated_at = now()
			 WHERE id = $1`,
			id, lastErr, backoff.Seconds())
		return expectAffected(tag, err, "job missing")
	}
	tag, err := r.pool.Exec(ctx,
		`UPDATE jobs SET status = 'failed', last_error = $2, updated_at = now() WHERE id = $1`,
		id, lastErr)
	return expectAffected(tag, err, "job missing")
}
