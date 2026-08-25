package handlers

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/apa/backend/internal/domain"
	"github.com/apa/backend/internal/domain/task"
)

func (h *Handlers) ListTasks(w http.ResponseWriter, r *http.Request) {
	actor, err := h.actorUser(r)
	if err != nil {
		writeError(w, r, h.deps.Log, err)
		return
	}
	if r.URL.Query().Get("available") == "true" {
		available, aerr := h.deps.Tasks.ListAvailable(r.Context(), actor, queryInt(r, "limit", 100))
		if aerr != nil {
			writeError(w, r, h.deps.Log, aerr)
			return
		}
		writeData(w, http.StatusOK, available)
		return
	}
	mine := r.URL.Query().Get("mine") == "true"
	status := r.URL.Query().Get("status")
	tasks, err := h.deps.Tasks.List(r.Context(), actor, mine, status, queryInt(r, "limit", 100))
	if err != nil {
		writeError(w, r, h.deps.Log, err)
		return
	}
	writeData(w, http.StatusOK, tasks)
}

func (h *Handlers) GetTask(w http.ResponseWriter, r *http.Request) {
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
	taskID, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, r, h.deps.Log, domain.Invalid("id", "invalid task id"))
		return
	}
	t, err := h.deps.Tasks.Get(r.Context(), orgID, taskID)
	if err != nil {
		writeError(w, r, h.deps.Log, err)
		return
	}
	writeData(w, http.StatusOK, t)
}

type patchTaskRequest struct {
	Action   string `json:"action"`
	Notes    string `json:"notes"`
	Guidance string `json:"guidance"`
}

func (h *Handlers) PatchTask(w http.ResponseWriter, r *http.Request) {
	actor, err := h.actorUser(r)
	if err != nil {
		writeError(w, r, h.deps.Log, err)
		return
	}
	taskID, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, r, h.deps.Log, domain.Invalid("id", "invalid task id"))
		return
	}
	var req patchTaskRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, r, h.deps.Log, err)
		return
	}

	var updated *task.Task
	switch req.Action {
	case "claim":
		updated, err = h.deps.Tasks.Claim(r.Context(), actor, taskID)
	case "start":
		updated, err = h.deps.Tasks.Start(r.Context(), actor, taskID)
	case "complete":
		updated, err = h.deps.Tasks.Complete(r.Context(), actor, taskID, req.Notes)
	case "resume":
		updated, err = h.deps.Tasks.Resume(r.Context(), actor, taskID, req.Guidance)
	default:
		err = domain.Invalid("action", "باید start، complete یا resume باشد")
	}
	if err != nil {
		writeError(w, r, h.deps.Log, err)
		return
	}
	writeData(w, http.StatusOK, map[string]any{"task": updated, "message": taskActionMessage(req.Action)})
}

type assignTaskRequest struct {
	UserID string `json:"userId"`
}

func (h *Handlers) AssignTask(w http.ResponseWriter, r *http.Request) {
	actor, err := h.actorUser(r)
	if err != nil {
		writeError(w, r, h.deps.Log, err)
		return
	}
	taskID, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, r, h.deps.Log, domain.Invalid("id", "invalid task id"))
		return
	}
	var req assignTaskRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, r, h.deps.Log, err)
		return
	}
	userID, err := uuid.Parse(req.UserID)
	if err != nil {
		writeError(w, r, h.deps.Log, domain.Invalid("userId", "invalid user id"))
		return
	}
	updated, err := h.deps.Tasks.Assign(r.Context(), actor, taskID, userID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) || errors.Is(err, domain.ErrForbidden) ||
			errors.Is(err, domain.ErrInvalidState) {
			writeError(w, r, h.deps.Log, err)
			return
		}
		writeError(w, r, h.deps.Log, err)
		return
	}
	writeData(w, http.StatusOK, updated)
}

func taskActionMessage(action string) string {
	switch action {
	case "claim":
		return "وظیفه به شما تخصیص یافت؛ حالا می‌توانید شروع کنید."
	case "start":
		return "وظیفه شروع شد."
	case "complete":
		return "وظیفه تکمیل شد. بررسی هوش مصنوعی در جریان است."
	default:
		return "وظیفه پس از راهنمایی ادامه یافت."
	}
}
