package repository

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/apa/backend/internal/domain/skill"
)

type Skills struct {
	pool *pgxpool.Pool
}

func NewSkills(pool *pgxpool.Pool) *Skills {
	return &Skills{pool: pool}
}

func (r *Skills) Create(ctx context.Context, sk *skill.Skill) error {
	err := r.pool.QueryRow(ctx,
		`INSERT INTO skills (org_id, name, description, keywords)
		 VALUES ($1,$2,$3,$4)
		 ON CONFLICT (org_id, name) DO UPDATE SET
		     description = EXCLUDED.description,
		     keywords = EXCLUDED.keywords
		 RETURNING id, created_at`,
		sk.OrgID, sk.Name, sk.Description, sk.Keywords,
	).Scan(&sk.ID, &sk.CreatedAt)
	return mapErr(err)
}

func (r *Skills) List(ctx context.Context, orgID uuid.UUID) ([]skill.Skill, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, org_id, name, description, keywords, created_at
		 FROM skills WHERE org_id = $1 ORDER BY name`, orgID)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()

	result := []skill.Skill{}
	for rows.Next() {
		var sk skill.Skill
		if err := rows.Scan(&sk.ID, &sk.OrgID, &sk.Name, &sk.Description, &sk.Keywords, &sk.CreatedAt); err != nil {
			return nil, mapErr(err)
		}
		result = append(result, sk)
	}
	return result, rows.Err()
}

func (r *Skills) Count(ctx context.Context, orgID uuid.UUID) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx,
		`SELECT count(*) FROM skills WHERE org_id = $1`, orgID).Scan(&n)
	return n, mapErr(err)
}

func (r *Skills) ExistsByNames(ctx context.Context, orgID uuid.UUID, names []string) (map[string]bool, error) {
	existing := map[string]bool{}
	if len(names) == 0 {
		return existing, nil
	}
	lowered := make([]string, len(names))
	for i, n := range names {
		lowered[i] = strings.ToLower(strings.TrimSpace(n))
	}
	rows, err := r.pool.Query(ctx,
		`SELECT name FROM skills WHERE org_id = $1 AND lower(name) = ANY($2)`,
		orgID, lowered)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, mapErr(err)
		}
		existing[strings.ToLower(name)] = true
	}
	return existing, rows.Err()
}
