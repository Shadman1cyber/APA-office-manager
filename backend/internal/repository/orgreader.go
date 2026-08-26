package repository

import (
	"context"

	"github.com/google/uuid"

	"github.com/apa/backend/internal/ai"
	"github.com/apa/backend/internal/domain/knowledge"
	"github.com/apa/backend/internal/domain/user"
)

type OrgReader struct {
	users     *Users
	knowledge *Knowledge
	skills    *Skills
}

func NewOrgReader(users *Users, knowledge *Knowledge, skills *Skills) *OrgReader {
	return &OrgReader{users: users, knowledge: knowledge, skills: skills}
}

func (o *OrgReader) ListSkills(ctx context.Context, orgID uuid.UUID) ([]ai.SkillDetail, error) {
	list, err := o.skills.List(ctx, orgID)
	if err != nil {
		return nil, err
	}
	out := make([]ai.SkillDetail, 0, len(list))
	for _, sk := range list {
		out = append(out, ai.SkillDetail{Name: sk.Name, Description: sk.Description, Keywords: sk.Keywords})
	}
	return out, nil
}

func (o *OrgReader) ListUsers(ctx context.Context, orgID uuid.UUID) ([]user.User, error) {
	return o.users.List(ctx, orgID)
}

func (o *OrgReader) FindFacts(ctx context.Context, orgID uuid.UUID, kind knowledge.FactKind, subjects []string) ([]knowledge.Fact, error) {
	return o.knowledge.FindFacts(ctx, orgID, kind, subjects)
}
