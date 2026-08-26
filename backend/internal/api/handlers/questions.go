package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/apa/backend/internal/domain"
)

type answerQuestionRequest struct {
	Answer string `json:"answer"`
}

func (h *Handlers) ListQuestions(w http.ResponseWriter, r *http.Request) {
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
	status := r.URL.Query().Get("status")
	var workflowID *uuid.UUID
	if raw := r.URL.Query().Get("workflowId"); raw != "" {
		parsed, perr := parseUUID(raw)
		if perr != nil {
			writeError(w, r, h.deps.Log, domain.Invalid("workflowId", "شناسه گردش‌کار نامعتبر است"))
			return
		}
		workflowID = &parsed
	}
	questions, err := h.deps.Questions.List(r.Context(), orgID, status, workflowID)
	if err != nil {
		writeError(w, r, h.deps.Log, err)
		return
	}
	writeData(w, http.StatusOK, questions)
}

func (h *Handlers) AnswerQuestion(w http.ResponseWriter, r *http.Request) {
	actor, err := h.actorUser(r)
	if err != nil {
		writeError(w, r, h.deps.Log, err)
		return
	}
	questionID, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, r, h.deps.Log, domain.Invalid("id", "شناسه سؤال نامعتبر است"))
		return
	}
	var req answerQuestionRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, r, h.deps.Log, err)
		return
	}
	result, err := h.deps.Questions.Answer(r.Context(), actor, questionID, req.Answer)
	if err != nil {
		writeError(w, r, h.deps.Log, err)
		return
	}
	writeData(w, http.StatusOK, result)
}
