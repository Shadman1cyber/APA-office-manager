package handlers

import (
	"errors"
	"net/http"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"github.com/apa/backend/internal/domain"
	domainUser "github.com/apa/backend/internal/domain/user"
)

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type registerRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *Handlers) Register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, r, h.deps.Log, err)
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))

	fields := map[string]string{}
	if req.Name == "" {
		fields["name"] = "نام الزامی است"
	}
	if req.Email == "" || !strings.Contains(req.Email, "@") || !strings.Contains(req.Email, ".") {
		fields["email"] = "ایمیل معتبر وارد کنید"
	}
	if len(req.Password) < 8 {
		fields["password"] = "رمز عبور باید حداقل ۸ نویسه باشد"
	}
	if len(fields) > 0 {
		writeError(w, r, h.deps.Log, &domain.ValidationError{Fields: fields})
		return
	}

	org, err := h.deps.Orgs.First(r.Context())
	if err != nil {
		writeError(w, r, h.deps.Log, err)
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		writeError(w, r, h.deps.Log, err)
		return
	}

	role := domainUser.RoleMember
	if count, cerr := h.deps.Users.Count(r.Context()); cerr == nil && count == 0 {
		role = domainUser.RoleManager
	}

	newUser := domainUser.User{
		OrgID:  org.ID,
		Email:  req.Email,
		Name:   req.Name,
		Role:   role,
		Skills: []string{},
	}
	if err := h.deps.Users.Create(r.Context(), &newUser, string(hash)); err != nil {
		writeError(w, r, h.deps.Log, err)
		return
	}

	token, err := h.deps.Tokens.Issue(middlewareIdentity(&newUser))
	if err != nil {
		writeError(w, r, h.deps.Log, err)
		return
	}
	writeData(w, http.StatusCreated, map[string]any{"token": token, "user": newUser})
}

func (h *Handlers) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, r, h.deps.Log, err)
		return
	}
	if req.Email == "" || req.Password == "" {
		writeError(w, r, h.deps.Log, domain.Invalid("email", "ایمیل و رمز عبور الزامی است"))
		return
	}

	u, hash, err := h.deps.Users.GetByEmail(r.Context(), req.Email)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeError(w, r, h.deps.Log, domain.ErrUnauthorized)
			return
		}
		writeError(w, r, h.deps.Log, err)
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.Password)); err != nil {
		writeError(w, r, h.deps.Log, domain.ErrUnauthorized)
		return
	}

	token, err := h.deps.Tokens.Issue(middlewareIdentity(u))
	if err != nil {
		writeError(w, r, h.deps.Log, err)
		return
	}
	writeData(w, http.StatusOK, map[string]any{"token": token, "user": u})
}

func (h *Handlers) Me(w http.ResponseWriter, r *http.Request) {
	id, ok := h.identity(r)
	if !ok {
		writeError(w, r, h.deps.Log, domain.ErrUnauthorized)
		return
	}
	userID, err := parseUUID(id.UserID)
	if err != nil {
		writeError(w, r, h.deps.Log, domain.ErrUnauthorized)
		return
	}
	u, err := h.deps.Users.Get(r.Context(), userID)
	if err != nil {
		writeError(w, r, h.deps.Log, err)
		return
	}
	writeData(w, http.StatusOK, u)
}

func (h *Handlers) Healthz(w http.ResponseWriter, r *http.Request) {
	if h.deps.Ping != nil {
		if err := h.deps.Ping(r.Context()); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{"status": "degraded", "database": "unreachable"})
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}
