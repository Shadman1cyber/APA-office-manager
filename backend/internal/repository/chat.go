package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"time"

	"github.com/apa/backend/internal/application"
)

type Chat struct {
	pool *pgxpool.Pool
}

func NewChat(pool *pgxpool.Pool) *Chat {
	return &Chat{pool: pool}
}

func (r *Chat) Append(ctx context.Context, m *application.ChatMessage) error {
	err := r.pool.QueryRow(ctx,
		`INSERT INTO chat_messages (org_id, user_id, role, text, action, workflow_id, question_id)
		 VALUES ($1,$2,$3,$4,$5,$6,$7)
		 RETURNING id, created_at`,
		m.OrgID, m.UserID, string(m.Role), m.Text, m.Action, m.WorkflowID, m.QuestionID,
	).Scan(&m.ID, &m.CreatedAt)
	return mapErr(err)
}

func (r *Chat) ListByUser(ctx context.Context, orgID, userID uuid.UUID, limit int) ([]*application.ChatMessage, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, org_id, user_id, role, text, action, workflow_id, question_id, created_at
		 FROM chat_messages
		 WHERE org_id = $1 AND user_id = $2
		 ORDER BY id DESC
		 LIMIT $3`, orgID, userID, limit)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()

	result := []*application.ChatMessage{}
	for rows.Next() {
		var msg application.ChatMessage
		var role string
		if err := rows.Scan(
			&msg.ID, &msg.OrgID, &msg.UserID, &role, &msg.Text,
			&msg.Action, &msg.WorkflowID, &msg.QuestionID, &msg.CreatedAt,
		); err != nil {
			return nil, mapErr(err)
		}
		msg.Role = application.ChatRole(role)
		result = append(result, &msg)
	}
	if len(result) == 0 {
		return result, rows.Err()
	}

	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}
	return result, rows.Err()
}

const tehranZone = "Asia/Tehran"

func (r *Chat) ListByUserOnDay(ctx context.Context, orgID, userID uuid.UUID, dayStart, dayEnd time.Time) ([]*application.ChatMessage, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, org_id, user_id, role, text, action, workflow_id, question_id, created_at
		 FROM chat_messages
		 WHERE org_id = $1 AND user_id = $2 AND created_at >= $3 AND created_at < $4
		 ORDER BY id ASC`, orgID, userID, dayStart, dayEnd)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()

	result := []*application.ChatMessage{}
	for rows.Next() {
		var msg application.ChatMessage
		var role string
		if err := rows.Scan(
			&msg.ID, &msg.OrgID, &msg.UserID, &role, &msg.Text,
			&msg.Action, &msg.WorkflowID, &msg.QuestionID, &msg.CreatedAt,
		); err != nil {
			return nil, mapErr(err)
		}
		msg.Role = application.ChatRole(role)
		result = append(result, &msg)
	}
	return result, rows.Err()
}

func (r *Chat) ListDays(ctx context.Context, orgID, userID uuid.UUID) ([]*application.ChatDaySummary, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT to_char(t.day, 'YYYY-MM-DD'), t.cnt::int, p.text
		 FROM (
		   SELECT (created_at AT TIME ZONE 'Asia/Tehran')::date AS day,
		          count(*) OVER w AS cnt,
		          row_number() OVER (PARTITION BY (created_at AT TIME ZONE 'Asia/Tehran')::date ORDER BY id ASC) AS rn
		   FROM chat_messages
		   WHERE org_id = $1 AND user_id = $2
		   WINDOW w AS (PARTITION BY (created_at AT TIME ZONE 'Asia/Tehran')::date)
		 ) t
		 JOIN LATERAL (
		   SELECT c.text FROM chat_messages c
		   WHERE c.user_id = $2 AND (c.created_at AT TIME ZONE 'Asia/Tehran')::date = t.day AND c.role = 'user'
		   ORDER BY c.id ASC LIMIT 1
		 ) p ON true
		 WHERE t.rn = 1
		 ORDER BY t.day DESC
		 LIMIT 90`, orgID, userID)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()

	result := []*application.ChatDaySummary{}
	for rows.Next() {
		var d application.ChatDaySummary
		if err := rows.Scan(&d.Day, &d.Count, &d.Preview); err != nil {
			return nil, mapErr(err)
		}
		result = append(result, &d)
	}
	return result, rows.Err()
}
