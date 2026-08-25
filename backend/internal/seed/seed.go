package seed

import (
	"context"
	"log/slog"

	"golang.org/x/crypto/bcrypt"

	"github.com/apa/backend/internal/application"
	"github.com/apa/backend/internal/domain/knowledge"
	"github.com/apa/backend/internal/domain/user"
)

const DefaultPassword = "password123"

type Deps struct {
	Orgs      application.OrganizationRepository
	Users     application.UserRepository
	Knowledge application.KnowledgeRepository
	Bus       *application.Bus
	Log       *slog.Logger
}

func Run(ctx context.Context, deps Deps) error {
	userCount, err := deps.Users.Count(ctx)
	if err != nil {
		return err
	}
	if userCount > 0 {
		return nil
	}

	org, err := deps.Orgs.Create(ctx, "Acme Corp")
	if err != nil {
		return err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(DefaultPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	seedUsers := []struct {
		email  string
		name   string
		role   user.Role
		skills []string
	}{
		{"sara@acme.test", "Sara (Manager)", user.RoleManager, []string{"writing", "review"}},
		{"ali@acme.test", "Ali Hassan", user.RoleMember, []string{"security", "statistics", "incident-response"}},
		{"mina@acme.test", "Mina Rahimi", user.RoleMember, []string{"writing", "communications"}},
	}

	ids := map[string]user.User{}
	for _, su := range seedUsers {
		u := &user.User{
			OrgID:  org.ID,
			Email:  su.email,
			Name:   su.name,
			Role:   su.role,
			Skills: su.skills,
		}
		if err := deps.Users.Create(ctx, u, string(hash)); err != nil {
			return err
		}
		ids[su.name] = *u
	}

	minaID := ids["Mina Rahimi"].ID
	fact, err := knowledge.NewFact(org.ID, knowledge.KindTopicOwner, "communications", minaID, 0.7, knowledge.SourceSeeded, "Seeded from HR directory: Mina leads internal communications.")
	if err != nil {
		return err
	}
	if err := deps.Knowledge.UpsertFact(ctx, fact); err != nil {
		return err
	}

	deps.Log.InfoContext(ctx, "seeded demo organization",
		slog.String("org", org.Name),
		slog.Int("users", len(seedUsers)),
		slog.String("password", DefaultPassword),
	)
	return nil
}
