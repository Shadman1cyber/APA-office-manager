package user

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

type Role string

const (
	RoleAdmin    Role = "admin"
	RoleManager  Role = "manager"
	RoleMember   Role = "member"
)

func (r Role) Valid() bool {
	return r == RoleAdmin || r == RoleManager || r == RoleMember
}

func (r Role) CanApprove() bool {
	return r == RoleAdmin || r == RoleManager
}

type User struct {
	ID           uuid.UUID `json:"id"`
	OrgID        uuid.UUID `json:"orgId"`
	Email        string    `json:"email"`
	Name         string    `json:"name"`
	Role         Role      `json:"role"`
	Skills       []string  `json:"skills"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"createdAt"`
}

func (u *User) HasSkill(skill string) bool {
	skill = strings.ToLower(strings.TrimSpace(skill))
	for _, s := range u.Skills {
		if strings.ToLower(s) == skill {
			return true
		}
	}
	return false
}
