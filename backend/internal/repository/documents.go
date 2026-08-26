package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/apa/backend/internal/domain/document"
)

type Documents struct {
	pool *pgxpool.Pool
}

func NewDocuments(pool *pgxpool.Pool) *Documents {
	return &Documents{pool: pool}
}

const documentColumns = `d.id, d.org_id, d.task_id, d.workflow_id, d.author_id, u.name,
	d.title, d.body, d.source_notes, d.status, d.created_at, d.updated_at`

const documentFrom = `FROM documents d LEFT JOIN users u ON u.id = d.author_id`

func scanDocument(row rowScanner) (*document.Document, error) {
	var d document.Document
	var status string
	if err := row.Scan(
		&d.ID, &d.OrgID, &d.TaskID, &d.WorkflowID, &d.AuthorID, &d.AuthorName,
		&d.Title, &d.Body, &d.SourceNotes, &status, &d.CreatedAt, &d.UpdatedAt,
	); err != nil {
		return nil, mapErr(err)
	}
	d.Status = document.Status(status)
	return &d, nil
}

func (r *Documents) Create(ctx context.Context, d *document.Document) error {
	err := r.pool.QueryRow(ctx,
		`INSERT INTO documents (org_id, task_id, workflow_id, author_id, title, body, source_notes, status)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		 RETURNING id, created_at, updated_at`,
		d.OrgID, d.TaskID, d.WorkflowID, d.AuthorID, d.Title, d.Body, d.SourceNotes, string(d.Status),
	).Scan(&d.ID, &d.CreatedAt, &d.UpdatedAt)
	return mapErr(err)
}

func (r *Documents) Get(ctx context.Context, orgID, id uuid.UUID) (*document.Document, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT `+documentColumns+` `+documentFrom+` WHERE d.id = $1 AND d.org_id = $2`, id, orgID)
	return scanDocument(row)
}

func (r *Documents) UpdateResult(ctx context.Context, id uuid.UUID, title, body string, status document.Status) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE documents SET title = $2, body = $3, status = $4, updated_at = now() WHERE id = $1`,
		id, title, body, string(status))
	return expectAffected(tag, err, "سند یافت نشد")
}

func (r *Documents) ListByOrg(ctx context.Context, orgID uuid.UUID, limit int) ([]*document.Document, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+documentColumns+` `+documentFrom+`
		 WHERE d.org_id = $1 ORDER BY d.created_at DESC LIMIT $2`, orgID, limit)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	return collectDocuments(rows)
}

func (r *Documents) ListByAuthor(ctx context.Context, orgID, authorID uuid.UUID, limit int) ([]*document.Document, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+documentColumns+` `+documentFrom+`
		 WHERE d.org_id = $1 AND d.author_id = $2 ORDER BY d.created_at DESC LIMIT $3`,
		orgID, authorID, limit)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	return collectDocuments(rows)
}

func collectDocuments(rows pgx.Rows) ([]*document.Document, error) {
	result := []*document.Document{}
	for rows.Next() {
		d, err := scanDocument(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, d)
	}
	return result, rows.Err()
}
