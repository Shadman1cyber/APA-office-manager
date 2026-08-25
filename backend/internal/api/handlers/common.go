package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/google/uuid"

	"github.com/apa/backend/internal/api/middleware"
	"github.com/apa/backend/internal/domain"
	"github.com/apa/backend/internal/domain/user"
)

func decodeJSON(r *http.Request, target any) error {
	if r.Body == nil {
		return domain.Invalid("body", "request body is required")
	}
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(target); err != nil {
		return domain.Invalid("body", "invalid JSON payload")
	}
	return nil
}

func parseUUID(raw string) (uuid.UUID, error) {
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, fmt.Errorf("%w: شناسهٔ نامعتبر %q", domain.ErrInvalidState, raw)
	}
	return id, nil
}

func middlewareIdentity(u *user.User) middleware.Identity {
	return middleware.Identity{
		UserID: u.ID.String(),
		OrgID:  u.OrgID.String(),
		Role:   u.Role,
		Name:   u.Name,
		Email:  u.Email,
	}
}

func queryInt(r *http.Request, key string, fallback int) int {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return fallback
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return fallback
	}
	return n
}

func (h *Handlers) actorUser(r *http.Request) (*user.User, error) {
	id, ok := middleware.IdentityFrom(r.Context())
	if !ok {
		return nil, domain.ErrUnauthorized
	}
	userID, err := parseUUID(id.UserID)
	if err != nil {
		return nil, domain.ErrUnauthorized
	}
	u, err := h.deps.Users.Get(r.Context(), userID)
	if err != nil {
		return nil, err
	}
	if u.OrgID.String() != id.OrgID {
		return nil, domain.ErrUnauthorized
	}
	return u, nil
}
