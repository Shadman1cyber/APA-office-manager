package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/apa/backend/internal/domain/organization"
)

type Organizations struct {
	pool *pgxpool.Pool
}

func NewOrganizations(pool *pgxpool.Pool) *Organizations {
	return &Organizations{pool: pool}
}

func (r *Organizations) Create(ctx context.Context, name string) (*organization.Organization, error) {
	org := &organization.Organization{Name: name}
	err := r.pool.QueryRow(ctx,
		`INSERT INTO organizations (name) VALUES ($1) RETURNING id, created_at`, name,
	).Scan(&org.ID, &org.CreatedAt)
	return org, mapErr(err)
}

func (r *Organizations) First(ctx context.Context) (*organization.Organization, error) {
	var org organization.Organization
	err := r.pool.QueryRow(ctx,
		`SELECT id, name, created_at FROM organizations ORDER BY created_at LIMIT 1`,
	).Scan(&org.ID, &org.Name, &org.CreatedAt)
	if err != nil {
		return nil, mapErr(err)
	}
	return &org, nil
}

func (r *Organizations) Count(ctx context.Context) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx, `SELECT count(*) FROM organizations`).Scan(&n)
	return n, mapErr(err)
}
