package handlers

import (
	"net/http"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"github.com/go-chi/chi/v5"

	"github.com/apa/backend/internal/domain"
	"github.com/apa/backend/internal/domain/user"
)

func (h *Handlers) ListEmployees(w http.ResponseWriter, r *http.Request) {
	id, ok := h.identity(r)
	if !ok {
		writeError(w, r, h.deps.Log, domain.ErrUnauthorized)
		return
	}
	orgID, err := parseUUID(id.OrgID)
	if err != nil {
		writeError(w, r, h.deps.Log, domain.ErrUnauthorized)
		return
	}
	employees, err := h.deps.Users.List(r.Context(), orgID)
	if err != nil {
		writeError(w, r, h.deps.Log, err)
		return
	}
	writeData(w, http.StatusOK, employees)
}

type createEmployeeRequest struct {
	Name     string   `json:"name"`
	Email    string   `json:"email"`
	Password string   `json:"password"`
	Role     string   `json:"role"`
	Skills   []string `json:"skills"`
}

func (h *Handlers) CreateEmployee(w http.ResponseWriter, r *http.Request) {
	actor, err := h.actorUser(r)
	if err != nil {
		writeError(w, r, h.deps.Log, err)
		return
	}
	if !actor.Role.CanApprove() {
		writeError(w, r, h.deps.Log, domain.ErrForbidden)
		return
	}

	var req createEmployeeRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, r, h.deps.Log, err)
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	req.Role = strings.TrimSpace(strings.ToLower(req.Role))

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
	role := user.RoleMember
	if req.Role != "" && req.Role != string(user.RoleMember) && req.Role != string(user.RoleManager) {
		fields["role"] = "نقش باید مدیر یا عضو باشد"
	} else if req.Role == string(user.RoleManager) {
		role = user.RoleManager
	}
	skills := normalizeSkills(req.Skills)
	if len(fields) > 0 {
		writeError(w, r, h.deps.Log, &domain.ValidationError{Fields: fields})
		return
	}

	org, err := h.deps.Orgs.First(r.Context())
	if err != nil {
		writeError(w, r, h.deps.Log, err)
		return
	}
	hash, err := bcryptHash(req.Password)
	if err != nil {
		writeError(w, r, h.deps.Log, err)
		return
	}

	newUser := user.User{
		OrgID:  org.ID,
		Email:  req.Email,
		Name:   req.Name,
		Role:   role,
		Skills: skills,
	}
	if err := h.deps.Users.Create(r.Context(), &newUser, hash); err != nil {
		writeError(w, r, h.deps.Log, err)
		return
	}
	writeData(w, http.StatusCreated, newUser)
}

type updateEmployeeRequest struct {
	Role   *string  `json:"role"`
	Skills []string `json:"skills"`
}

func (h *Handlers) UpdateEmployee(w http.ResponseWriter, r *http.Request) {
	actor, err := h.actorUser(r)
	if err != nil {
		writeError(w, r, h.deps.Log, err)
		return
	}
	if !actor.Role.CanApprove() {
		writeError(w, r, h.deps.Log, domain.ErrForbidden)
		return
	}
	userID, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, r, h.deps.Log, domain.Invalid("id", "شناسه کاربر نامعتبر است"))
		return
	}

	var req updateEmployeeRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, r, h.deps.Log, err)
		return
	}

	var role *user.Role
	if req.Role != nil {
		parsed := user.Role(strings.TrimSpace(strings.ToLower(*req.Role)))
		if parsed != user.RoleManager && parsed != user.RoleMember {
			writeError(w, r, h.deps.Log, domain.Invalid("role", "نقش باید مدیر یا عضو باشد"))
			return
		}
		role = &parsed
	}
	var skills []string
	if req.Skills != nil {
		skills = normalizeSkills(req.Skills)
	}
	if role == nil && req.Skills == nil {
		writeError(w, r, h.deps.Log, domain.Invalid("body", "نقش و/یا مهارت‌ها را برای به‌روزرسانی مشخص کنید"))
		return
	}
	if userID == actor.ID && role != nil && *role != actor.Role {
		writeError(w, r, h.deps.Log, domain.Invalid("role", "نمی‌توانید نقش خودتان را تغییر دهید"))
		return
	}

	if err := h.deps.Users.UpdateRoleSkills(r.Context(), userID, role, skills); err != nil {
		writeError(w, r, h.deps.Log, err)
		return
	}
	updated, err := h.deps.Users.Get(r.Context(), userID)
	if err != nil {
		writeError(w, r, h.deps.Log, err)
		return
	}
	writeData(w, http.StatusOK, updated)
}

func bcryptHash(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(hash), err
}

func normalizeSkills(raw []string) []string {
	out := []string{}
	for _, s := range raw {
		trimmed := strings.ToLower(strings.TrimSpace(s))
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}
