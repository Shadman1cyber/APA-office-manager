package handlers

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/apa/backend/internal/domain"
	"github.com/apa/backend/internal/domain/skill"
)

func (h *Handlers) ListSkills(w http.ResponseWriter, r *http.Request) {
	actor, err := h.actorUser(r)
	if err != nil {
		writeError(w, r, h.deps.Log, err)
		return
	}
	skills, err := h.deps.Skills.List(r.Context(), actor.OrgID)
	if err != nil {
		writeError(w, r, h.deps.Log, err)
		return
	}
	writeData(w, http.StatusOK, skills)
}

type createSkillRequest struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Keywords    []string `json:"keywords"`
}

func (h *Handlers) CreateSkill(w http.ResponseWriter, r *http.Request) {
	actor, err := h.actorUser(r)
	if err != nil {
		writeError(w, r, h.deps.Log, err)
		return
	}
	if !actor.Role.CanApprove() {
		writeError(w, r, h.deps.Log, fmt.Errorf("%w: تعریف مهارت جدید فقط برای مدیران مجاز است", domain.ErrForbidden))
		return
	}
	var req createSkillRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, r, h.deps.Log, err)
		return
	}
	normalizedName := skill.NormalizeName(req.Name)
	if exists, eerr := h.deps.Skills.ExistsByNames(r.Context(), actor.OrgID, []string{normalizedName}); eerr == nil && exists[normalizedName] {
		writeError(w, r, h.deps.Log, fmt.Errorf("%w: مهارت «%s» قبلاً تعریف شده است", domain.ErrAlreadyExists, normalizedName))
		return
	}
	sk, serr := skill.New(actor.OrgID, req.Name, req.Description, req.Keywords)
	if serr != nil {
		writeError(w, r, h.deps.Log, serr)
		return
	}
	if cerr := h.deps.Skills.Create(r.Context(), sk); cerr != nil {
		writeError(w, r, h.deps.Log, cerr)
		return
	}
	writeData(w, http.StatusCreated, sk)
}

// validateSkillsAgainstCatalog ensures every submitted skill name is defined by the manager.
func (h *Handlers) validateSkillsAgainstCatalog(r *http.Request, orgID uuid.UUID, names []string) error {
	if len(names) == 0 {
		return nil
	}
	existing, err := h.deps.Skills.ExistsByNames(r.Context(), orgID, names)
	if err != nil {
		return err
	}
	unknown := []string{}
	for _, n := range names {
		if !existing[strings.ToLower(strings.TrimSpace(n))] {
			unknown = append(unknown, n)
		}
	}
	if len(unknown) > 0 {
		return domain.Invalid("skills",
			"این مهارت‌ها در فهرست سازمان تعریف نشده‌اند: "+strings.Join(unknown, "، ")+" — ابتدا از بخش مهارت‌ها تعریف کنید")
	}
	return nil
}
