package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/apa/backend/internal/domain"
	"github.com/apa/backend/internal/domain/user"
)

type Users struct {
	pool *pgxpool.Pool
}

func NewUsers(pool *pgxpool.Pool) *Users {
	return &Users{pool: pool}
}

func (r *Users) Create(ctx context.Context, u *user.User, passwordHash string) error {
	err := r.pool.QueryRow(ctx,
		`INSERT INTO users (org_id, email, name, role, password_hash, skills)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, created_at`,
		u.OrgID, u.Email, u.Name, string(u.Role), passwordHash, u.Skills,
	).Scan(&u.ID, &u.CreatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return fmt.Errorf("%w", domain.ErrEmailTaken)
		}
	}
	return mapErr(err)
}

func (r *Users) Get(ctx context.Context, id uuid.UUID) (*user.User, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, org_id, email, name, role, skills, '' AS password_hash, created_at
		 FROM users WHERE id = $1`, id)
	return scanUser(row)
}

func (r *Users) GetByEmail(ctx context.Context, email string) (*user.User, string, error) {
	var u user.User
	var role string
	err := r.pool.QueryRow(ctx,
		`SELECT id, org_id, email, name, role, skills, password_hash, created_at
		 FROM users WHERE lower(email) = lower($1)`, email,
	).Scan(&u.ID, &u.OrgID, &u.Email, &u.Name, &role, &u.Skills, &u.PasswordHash, &u.CreatedAt)
	if err != nil {
		return nil, "", mapErr(err)
	}
	u.Role = user.Role(role)
	return &u, u.PasswordHash, nil
}

func (r *Users) List(ctx context.Context, orgID uuid.UUID) ([]user.User, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, org_id, email, name, role, skills, '' AS password_hash, created_at
		 FROM users WHERE org_id = $1 ORDER BY name`, orgID)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()

	users := []user.User{}
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, *u)
	}
	return users, rows.Err()
}

func (r *Users) Count(ctx context.Context) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx, `SELECT count(*) FROM users`).Scan(&n)
	return n, mapErr(err)
}

func scanUser(row rowScanner) (*user.User, error) {
	var u user.User
	var role string
	if err := row.Scan(&u.ID, &u.OrgID, &u.Email, &u.Name, &role, &u.Skills, &u.PasswordHash, &u.CreatedAt); err != nil {
		return nil, mapErr(err)
	}
	u.Role = user.Role(role)
	return &u, nil
}

func (r *Users) UpdateRoleSkills(ctx context.Context, id uuid.UUID, role *user.Role, skills []string) error {
	var roleArg any
	if role != nil {
		roleArg = string(*role)
	}
	var skillsArg any
	if skills != nil {
		skillsArg = skills
	}
	tag, err := r.pool.Exec(ctx,
		`UPDATE users SET role = COALESCE($2, role), skills = COALESCE($3, skills) WHERE id = $1`,
		id, roleArg, skillsArg)
	return expectAffected(tag, err, "user not found")
}
