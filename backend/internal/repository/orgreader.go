package repository

import (
	"context"

	"github.com/google/uuid"

	"github.com/apa/backend/internal/domain/knowledge"
	"github.com/apa/backend/internal/domain/user"
)

type OrgReader struct {
	users     *Users
	knowledge *Knowledge
}

func NewOrgReader(users *Users, knowledge *Knowledge) *OrgReader {
	return &OrgReader{users: users, knowledge: knowledge}
}

func (o *OrgReader) ListUsers(ctx context.Context, orgID uuid.UUID) ([]user.User, error) {
	return o.users.List(ctx, orgID)
}

func (o *OrgReader) FindFacts(ctx context.Context, orgID uuid.UUID, kind knowledge.FactKind, subjects []string) ([]knowledge.Fact, error) {
	return o.knowledge.FindFacts(ctx, orgID, kind, subjects)
}
