package repository

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/apa/backend/internal/domain/knowledge"
	"github.com/apa/backend/internal/domain/user"
)

type Knowledge struct {
	pool *pgxpool.Pool
}

func NewKnowledge(pool *pgxpool.Pool) *Knowledge {
	return &Knowledge{pool: pool}
}

const factColumns = `f.id, f.org_id, f.kind, f.subject, f.person_id, u.name, f.confidence, f.source, f.evidence, f.evidence_count, f.created_at, f.updated_at`

const factFrom = `FROM knowledge_facts f LEFT JOIN users u ON u.id = f.person_id`

func scanFactRow(row rowScanner) (*knowledge.Fact, error) {
	var f knowledge.Fact
	var kind, source string
	var personName *string
	if err := row.Scan(
		&f.ID, &f.OrgID, &kind, &f.Subject, &f.PersonID, &personName, &f.Confidence,
		&source, &f.Evidence, &f.EvidenceCount, &f.CreatedAt, &f.UpdatedAt,
	); err != nil {
		return nil, mapErr(err)
	}
	f.Kind = knowledge.FactKind(kind)
	f.Source = knowledge.Source(source)
	if personName != nil {
		f.PersonName = *personName
	}
	return &f, nil
}

func collectFacts(rows pgx.Rows) ([]knowledge.Fact, error) {
	result := []knowledge.Fact{}
	for rows.Next() {
		f, err := scanFactRow(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, *f)
	}
	return result, rows.Err()
}

func (r *Knowledge) UpsertFact(ctx context.Context, f *knowledge.Fact) error {
	err := r.pool.QueryRow(ctx,
		`INSERT INTO knowledge_facts (org_id, kind, subject, person_id, confidence, source, evidence, evidence_count)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		 ON CONFLICT (kind, subject, person_id) DO UPDATE SET
		     confidence = EXCLUDED.confidence,
		     source = EXCLUDED.source,
		     evidence = EXCLUDED.evidence,
		     evidence_count = EXCLUDED.evidence_count,
		     updated_at = now()
		 RETURNING id, created_at, updated_at`,
		f.OrgID, string(f.Kind), f.Subject, f.PersonID, f.Confidence,
		string(f.Source), f.Evidence, f.EvidenceCount,
	).Scan(&f.ID, &f.CreatedAt, &f.UpdatedAt)
	return mapErr(err)
}

func (r *Knowledge) FindFacts(ctx context.Context, orgID uuid.UUID, kind knowledge.FactKind, subjects []string) ([]knowledge.Fact, error) {
	query := `SELECT ` + factColumns + ` ` + factFrom + ` WHERE f.org_id = $1 AND f.kind = $2`
	args := []any{orgID, string(kind)}
	argN := 3
	if len(subjects) > 0 {
		query += " AND lower(subject) = ANY($" + strconv.Itoa(argN) + ")"
		args = append(args, loweredAll(subjects))
		argN++
	}
	query += " ORDER BY confidence DESC"

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	return collectFacts(rows)
}

func (r *Knowledge) FindFact(ctx context.Context, orgID uuid.UUID, kind knowledge.FactKind, subject string, personID uuid.UUID) (*knowledge.Fact, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT `+factColumns+` `+factFrom+`
		 WHERE f.org_id = $1 AND f.kind = $2 AND lower(f.subject) = $3 AND f.person_id = $4`,
		orgID, string(kind), strings.ToLower(subject), personID)
	return scanFactRow(row)
}

func (r *Knowledge) ListAllFacts(ctx context.Context, orgID uuid.UUID) ([]knowledge.Fact, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+factColumns+` `+factFrom+` WHERE f.org_id = $1 ORDER BY f.confidence DESC`, orgID)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	return collectFacts(rows)
}

func (r *Knowledge) CountFacts(ctx context.Context, orgID uuid.UUID) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx,
		`SELECT count(*) FROM knowledge_facts WHERE org_id = $1`, orgID).Scan(&n)
	return n, mapErr(err)
}

const profileQuery = `SELECT u.id, u.org_id, u.email, u.name, u.role, u.skills, u.created_at,
	COALESCE(json_agg(json_build_object('subject', f.subject, 'confidence', f.confidence, 'evidenceCount', f.evidence_count)
	    ORDER BY f.confidence DESC) FILTER (WHERE f.id IS NOT NULL), '[]'::json)
FROM users u
LEFT JOIN knowledge_facts f ON f.person_id = u.id AND f.kind = 'topic_owner'
WHERE u.org_id = $1
GROUP BY u.id
ORDER BY u.name`

func (r *Knowledge) PeopleProfiles(ctx context.Context, orgID uuid.UUID) ([]knowledge.PersonProfile, error) {
	rows, err := r.pool.Query(ctx, profileQuery, orgID)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()

	result := []knowledge.PersonProfile{}
	for rows.Next() {
		var p knowledge.PersonProfile
		var role string
		var topicsBytes []byte
		if err := rows.Scan(&p.ID, &p.OrgID, &p.Email, &p.Name, &role, &p.Skills, &p.CreatedAt, &topicsBytes); err != nil {
			return nil, mapErr(err)
		}
		p.Role = user.Role(role)
		p.OwnedTopics = []knowledge.OwnedTopic{}
		if len(topicsBytes) > 0 {
			if err := json.Unmarshal(topicsBytes, &p.OwnedTopics); err != nil {
				return nil, err
			}
		}
		result = append(result, p)
	}
	return result, rows.Err()
}

func loweredAll(values []string) []string {
	out := make([]string, len(values))
	for i, v := range values {
		out[i] = strings.ToLower(v)
	}
	return out
}
