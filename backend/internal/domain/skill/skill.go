package skill

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/apa/backend/internal/domain"
)

type Skill struct {
	ID          uuid.UUID `json:"id"`
	OrgID       uuid.UUID `json:"orgId"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Keywords    []string  `json:"keywords"`
	CreatedAt   time.Time `json:"createdAt"`
}

func NormalizeName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func New(orgID uuid.UUID, name, description string, keywords []string) (*Skill, error) {
	name = NormalizeName(name)
	if name == "" {
		return nil, domain.Invalid("name", "نام مهارت الزامی است")
	}
	description = strings.TrimSpace(description)
	if len([]rune(description)) < 10 {
		return nil, domain.Invalid("description", "برای هوش مصنوعی توضیح دهید این مهارت یعنی چه (حداقل ۱۰ نویسه)")
	}
	cleaned := []string{}
	for _, k := range keywords {
		k = strings.ToLower(strings.TrimSpace(k))
		if k != "" {
			cleaned = append(cleaned, k)
		}
	}
	if len(cleaned) == 0 {
		return nil, domain.Invalid("keywords", "حداقل یک کلیدواژه لازم است تا هوش مصنوعی مهارت را تشخیص دهد")
	}
	return &Skill{
		OrgID:       orgID,
		Name:        name,
		Description: description,
		Keywords:    cleaned,
	}, nil
}

func (s *Skill) Matches(textLower string) bool {
	for _, k := range s.Keywords {
		if k != "" && strings.Contains(textLower, k) {
			return true
		}
	}
	return false
}

var _ = fmt.Sprintf
