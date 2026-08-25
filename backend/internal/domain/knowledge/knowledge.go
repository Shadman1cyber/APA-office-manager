package knowledge

import (
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/apa/backend/internal/domain"
)

const (
	MinConfidence = 0.05
	MaxConfidence = 0.95
)

type FactKind string

const (
	KindTopicOwner FactKind = "topic_owner"
	KindSkill      FactKind = "skill"
)

func ParseKind(raw string) (FactKind, error) {
	k := FactKind(raw)
	switch k {
	case KindTopicOwner, KindSkill:
		return k, nil
	default:
		return "", fmt.Errorf("%w: unknown knowledge kind %q", domain.ErrInvalidState, raw)
	}
}

type Source string

const (
	SourceSeeded  Source = "seeded"
	SourceLearned Source = "learned"
)

type Fact struct {
	ID            uuid.UUID `json:"id"`
	OrgID         uuid.UUID `json:"orgId"`
	Kind          FactKind  `json:"kind"`
	Subject       string    `json:"subject"`
	PersonID      uuid.UUID `json:"personId"`
	PersonName    string    `json:"personName,omitempty"`
	Confidence    float64   `json:"confidence"`
	Source        Source    `json:"source"`
	Evidence      string    `json:"evidence"`
	EvidenceCount int       `json:"evidenceCount"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

func clamp(v float64) float64 {
	if v < MinConfidence {
		return MinConfidence
	}
	if v > MaxConfidence {
		return MaxConfidence
	}
	return v
}

func NewFact(orgID uuid.UUID, kind FactKind, subject string, personID uuid.UUID, confidence float64, source Source, evidence string) (*Fact, error) {
	if subject == "" {
		return nil, fmt.Errorf("%w: knowledge subject is empty", domain.ErrInsufficientData)
	}
	if personID == uuid.Nil {
		return nil, fmt.Errorf("%w: knowledge fact needs a person", domain.ErrInsufficientData)
	}
	now := time.Now().UTC()
	return &Fact{
		OrgID:         orgID,
		Kind:          kind,
		Subject:       subject,
		PersonID:      personID,
		Confidence:    clamp(confidence),
		Source:        source,
		Evidence:      evidence,
		EvidenceCount: 1,
		CreatedAt:     now,
		UpdatedAt:     now,
	}, nil
}

func (f *Fact) Reinforce(delta float64, evidence string) {
	f.Confidence = clamp(f.Confidence + delta)
	f.EvidenceCount++
	f.Evidence = evidence
	f.UpdatedAt = time.Now().UTC()
}
