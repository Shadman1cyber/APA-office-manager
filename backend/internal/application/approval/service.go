package approvalsvc

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/apa/backend/internal/application"
	"github.com/apa/backend/internal/domain/approval"
)

type Service struct {
	repo application.ApprovalRepository
	bus  *application.Bus
}

func NewService(repo application.ApprovalRepository, bus *application.Bus) *Service {
	return &Service{repo: repo, bus: bus}
}

func (s *Service) CreatePlanApproval(ctx context.Context, orgID, workflowID uuid.UUID, payload []byte) (*approval.Approval, error) {
	a, err := approval.NewPlan(orgID, workflowID, payload)
	if err != nil {
		return nil, err
	}
	if err := s.repo.Create(ctx, a); err != nil {
		return nil, err
	}
	return a, nil
}

func (s *Service) Decide(ctx context.Context, id uuid.UUID, status approval.Status, decidedBy uuid.UUID) error {
	return s.repo.Decide(ctx, id, status, decidedBy, time.Now().UTC())
}

func (s *Service) LatestPlan(ctx context.Context, workflowID uuid.UUID) (*approval.Approval, error) {
	return s.repo.LatestForWorkflow(ctx, workflowID, approval.TypePlan)
}
