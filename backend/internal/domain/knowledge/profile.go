package knowledge

import (
	"github.com/apa/backend/internal/domain/user"
)

type OwnedTopic struct {
	Subject       string  `json:"subject"`
	Confidence    float64 `json:"confidence"`
	EvidenceCount int     `json:"evidenceCount"`
}

type PersonProfile struct {
	user.User
	OwnedTopics []OwnedTopic `json:"ownedTopics"`
}
