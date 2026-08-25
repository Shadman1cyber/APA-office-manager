package handlers

import (
	"fmt"
	"net/http"

	"github.com/google/uuid"

	"github.com/apa/backend/internal/domain"
	"github.com/apa/backend/internal/domain/knowledge"
)

func (h *Handlers) requireManager(r *http.Request) (*uuid.UUID, error) {
	actor, err := h.actorUser(r)
	if err != nil {
		return nil, err
	}
	if !actor.Role.CanApprove() {
		return nil, fmt.Errorf("%w: دسترسی به دانش سازمانی فقط برای مدیران مجاز است", domain.ErrForbidden)
	}
	orgID := actor.OrgID
	return &orgID, nil
}

func (h *Handlers) KnowledgeOverview(w http.ResponseWriter, r *http.Request) {
	orgID, err := h.requireManager(r)
	if err != nil {
		writeError(w, r, h.deps.Log, err)
		return
	}
	overview, err := h.deps.Knowledge.Overview(r.Context(), *orgID)
	if err != nil {
		writeError(w, r, h.deps.Log, err)
		return
	}
	writeData(w, http.StatusOK, overview)
}

func (h *Handlers) ListPeople(w http.ResponseWriter, r *http.Request) {
	orgID, err := h.requireManager(r)
	if err != nil {
		writeError(w, r, h.deps.Log, err)
		return
	}
	people, err := h.deps.Knowledge.People(r.Context(), *orgID)
	if err != nil {
		writeError(w, r, h.deps.Log, err)
		return
	}
	writeData(w, http.StatusOK, people)
}

func (h *Handlers) ListFacts(w http.ResponseWriter, r *http.Request) {
	orgID, err := h.requireManager(r)
	if err != nil {
		writeError(w, r, h.deps.Log, err)
		return
	}
	subjects := []string{}
	if s := r.URL.Query().Get("subject"); s != "" {
		subjects = append(subjects, s)
	}
	facts, err := h.deps.Knowledge.Facts(r.Context(), *orgID, r.URL.Query().Get("kind"), subjects)
	if err != nil {
		writeError(w, r, h.deps.Log, err)
		return
	}
	writeData(w, http.StatusOK, facts)
}

type addFactRequest struct {
	Kind     string `json:"kind"`
	Subject  string `json:"subject"`
	PersonID string `json:"personId"`
	Evidence string `json:"evidence"`
}

func (h *Handlers) AddFact(w http.ResponseWriter, r *http.Request) {
	actor, err := h.actorUser(r)
	if err != nil {
		writeError(w, r, h.deps.Log, err)
		return
	}
	if !actor.Role.CanApprove() {
		writeError(w, r, h.deps.Log, domain.ErrForbidden)
		return
	}
	var req addFactRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, r, h.deps.Log, err)
		return
	}
	kind, err := knowledge.ParseKind(req.Kind)
	if err != nil {
		writeError(w, r, h.deps.Log, err)
		return
	}
	personID, err := uuid.Parse(req.PersonID)
	if err != nil {
		writeError(w, r, h.deps.Log, domain.Invalid("personId", "شناسه شخص نامعتبر است"))
		return
	}
	evidence := req.Evidence
	if evidence == "" {
		evidence = "ثبت‌شده دستی توسط " + actor.Name
	}
	fact, err := h.deps.Knowledge.Record(r.Context(), actor.OrgID, kind, req.Subject, personID, 0.6, knowledge.SourceLearned, evidence)
	if err != nil {
		writeError(w, r, h.deps.Log, err)
		return
	}
	writeData(w, http.StatusCreated, fact)
}
