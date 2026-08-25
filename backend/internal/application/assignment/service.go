package assignmentsvc

import (
	"fmt"

	"github.com/google/uuid"

	"github.com/apa/backend/internal/ai"
	"github.com/apa/backend/internal/domain/task"
)

func FromAI(ap ai.AssignmentProposal) (*task.Proposal, error) {
	proposal := &task.Proposal{
		Evidence:                  ap.Evidence,
		Confidence:                ap.Confidence,
		RequiresHumanConfirmation: ap.RequiresHumanConfirmation,
		CandidateName:             ap.CandidateName,
	}
	if ap.CandidateUserID != nil && *ap.CandidateUserID != "" {
		id, err := uuid.Parse(*ap.CandidateUserID)
		if err != nil {
			return nil, fmt.Errorf("invalid candidate user id: %w", err)
		}
		proposal.CandidateUserID = &id
	}
	if proposal.Evidence == nil {
		proposal.Evidence = []string{}
	}
	if err := proposal.Validate(); err != nil {
		return nil, err
	}
	return proposal, nil
}
