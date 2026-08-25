package knowledgesvc

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/apa/backend/internal/application"
	"github.com/apa/backend/internal/domain/knowledge"
	"github.com/apa/backend/internal/domain/user"
)

type Logger interface {
	WarnContext(ctx context.Context, msg string, args ...any)
	ErrorContext(ctx context.Context, msg string, args ...any)
}

const InitialLearnedConfidence = 0.55

type Service struct {
	repo  application.KnowledgeRepository
	users application.UserRepository
	bus   *application.Bus
	log   Logger
}

func NewService(repo application.KnowledgeRepository, users application.UserRepository, bus *application.Bus, log Logger) *Service {
	return &Service{repo: repo, users: users, bus: bus, log: log}
}

func (s *Service) Record(ctx context.Context, orgID uuid.UUID, kind knowledge.FactKind, subject string, personID uuid.UUID, confidence float64, source knowledge.Source, evidence string) (*knowledge.Fact, error) {
	f, err := knowledge.NewFact(orgID, kind, subject, personID, confidence, source, evidence)
	if err != nil {
		return nil, err
	}
	if err := s.repo.UpsertFact(ctx, f); err != nil {
		return nil, fmt.Errorf("save knowledge fact: %w", err)
	}
	return f, nil
}

func (s *Service) Reinforce(ctx context.Context, orgID uuid.UUID, subject string, personID uuid.UUID, delta float64, evidence string) (*knowledge.Fact, error) {
	existing, err := s.repo.FindFact(ctx, orgID, knowledge.KindTopicOwner, subject, personID)
	if err != nil {
		return nil, err
	}
	existing.Reinforce(delta, evidence)
	if err := s.repo.UpsertFact(ctx, existing); err != nil {
		return nil, fmt.Errorf("reinforce knowledge fact: %w", err)
	}
	return existing, nil
}

func (s *Service) LearnFromAnswer(ctx context.Context, orgID uuid.UUID, subject string, personID uuid.UUID, evidence string) (*knowledge.Fact, error) {
	existing, err := s.repo.FindFact(ctx, orgID, knowledge.KindTopicOwner, subject, personID)
	if err == nil && existing != nil {
		existing.Reinforce(0.08, evidence)
		if uerr := s.repo.UpsertFact(ctx, existing); uerr != nil {
			return nil, uerr
		}
		return existing, nil
	}
	f, cerr := knowledge.NewFact(orgID, knowledge.KindTopicOwner, subject, personID, InitialLearnedConfidence, knowledge.SourceLearned, evidence)
	if cerr != nil {
		return nil, cerr
	}
	if uerr := s.repo.UpsertFact(ctx, f); uerr != nil {
		return nil, uerr
	}
	return f, nil
}

func (s *Service) People(ctx context.Context, orgID uuid.UUID) ([]knowledge.PersonProfile, error) {
	return s.repo.PeopleProfiles(ctx, orgID)
}

func (s *Service) Facts(ctx context.Context, orgID uuid.UUID, kind string, subjects []string) ([]knowledge.Fact, error) {
	if kind == "" {
		return s.repo.ListAllFacts(ctx, orgID)
	}
	k, err := knowledge.ParseKind(kind)
	if err != nil {
		return nil, err
	}
	return s.repo.FindFacts(ctx, orgID, k, subjects)
}

func (s *Service) Overview(ctx context.Context, orgID uuid.UUID) (map[string]any, error) {
	profiles, err := s.repo.PeopleProfiles(ctx, orgID)
	if err != nil {
		return nil, err
	}
	factCount, err := s.repo.CountFacts(ctx, orgID)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"peopleCount": len(profiles),
		"factCount":   factCount,
	}, nil
}

func (s *Service) PersonByID(ctx context.Context, orgID, personID uuid.UUID) (*user.User, error) {
	return s.users.Get(ctx, personID)
}
