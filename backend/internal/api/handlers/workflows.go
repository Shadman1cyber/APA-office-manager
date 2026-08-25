package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/apa/backend/internal/application/chat"
	"github.com/apa/backend/internal/domain"
	"github.com/apa/backend/internal/domain/workflow"
)

func (h *Handlers) ListWorkflows(w http.ResponseWriter, r *http.Request) {
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
	actorID, err := parseUUID(id.UserID)
	if err != nil {
		writeError(w, r, h.deps.Log, domain.ErrUnauthorized)
		return
	}

	var workflows []*workflow.Workflow
	if id.Role.CanApprove() {
		workflows, err = h.deps.Workflows.List(r.Context(), orgID, queryInt(r, "limit", 50))
	} else {
		workflows, err = h.deps.Workflows.ListForUser(r.Context(), orgID, actorID)
	}
	if err != nil {
		writeError(w, r, h.deps.Log, err)
		return
	}
	writeData(w, http.StatusOK, workflows)
}

type createWorkflowRequest struct {
	Intent string `json:"intent"`
}

func (h *Handlers) CreateWorkflow(w http.ResponseWriter, r *http.Request) {
	actor, err := h.actorUser(r)
	if err != nil {
		writeError(w, r, h.deps.Log, err)
		return
	}
	if !actor.Role.CanApprove() {
		writeError(w, r, h.deps.Log, fmt.Errorf("%w: ثبت درخواست جدید فقط برای مدیران فعال است", domain.ErrForbidden))
		return
	}
	var req createWorkflowRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, r, h.deps.Log, err)
		return
	}
	view, err := h.deps.Workflows.Create(r.Context(), actor, req.Intent)
	if err != nil {
		writeError(w, r, h.deps.Log, err)
		return
	}
	reply := chatsvc.Reply{
		Text:       "یک گردش‌کار پیشنهادی با عنوان «" + view.Workflow.Title + "» و " + itoa(len(view.Tasks)) + " وظیفه ساختم.",
		Action:     "created",
		WorkflowID: &view.Workflow.ID,
		Workflow:   view,
	}
	if len(view.OpenQuestions()) > 0 {
		first := view.OpenQuestions()[0]
		qid := first.ID
		reply.QuestionID = &qid
		reply.Text += "\n\n" + first.Text
	} else {
		reply.Text += "\nبرای تأیید برنامه بگویید «تأیید»."
	}
	writeData(w, http.StatusCreated, reply)
}

func (h *Handlers) GetWorkflow(w http.ResponseWriter, r *http.Request) {
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
	wfID, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, r, h.deps.Log, domain.Invalid("id", "invalid workflow id"))
		return
	}
	view, err := h.deps.Workflows.Get(r.Context(), orgID, wfID)
	if err != nil {
		writeError(w, r, h.deps.Log, err)
		return
	}
	id2, _ := h.identity(r)
	if !id2.Role.CanApprove() {
		userID, uerr := parseUUID(id2.UserID)
		if uerr != nil {
			writeError(w, r, h.deps.Log, domain.ErrUnauthorized)
			return
		}
		involved, ierr := h.deps.Tasks.UserInvolvedInWorkflow(r.Context(), orgID, wfID, userID)
		if ierr != nil {
			writeError(w, r, h.deps.Log, ierr)
			return
		}
		if !involved {
			writeError(w, r, h.deps.Log, fmt.Errorf("%w: این گردش‌کار مربوط به شما نیست", domain.ErrForbidden))
			return
		}
	}
	writeData(w, http.StatusOK, view)
}

type rejectRequest struct {
	Reason string `json:"reason"`
}

func (h *Handlers) ApproveWorkflow(w http.ResponseWriter, r *http.Request) {
	actor, err := h.actorUser(r)
	if err != nil {
		writeError(w, r, h.deps.Log, err)
		return
	}
	wfID, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, r, h.deps.Log, domain.Invalid("id", "invalid workflow id"))
		return
	}
	view, assigned, err := h.deps.Workflows.Approve(r.Context(), actor, wfID)
	if err != nil {
		writeError(w, r, h.deps.Log, err)
		return
	}
	text := "تأیید شد؛ " + itoa(len(view.Tasks)) + " وظیفه ساخته شد."
	if len(assigned) > 0 {
		text += " تخصیص‌ها: " + joinStrings(assigned, "؛ ") + "."
	}
	writeData(w, http.StatusOK, map[string]any{
		"message":  text,
		"assigned": assigned,
		"workflow": view,
	})
}

func (h *Handlers) RejectWorkflow(w http.ResponseWriter, r *http.Request) {
	actor, err := h.actorUser(r)
	if err != nil {
		writeError(w, r, h.deps.Log, err)
		return
	}
	wfID, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, r, h.deps.Log, domain.Invalid("id", "invalid workflow id"))
		return
	}
	var req rejectRequest
	_ = decodeJSON(r, &req)
	view, err := h.deps.Workflows.Reject(r.Context(), actor, wfID, req.Reason)
	if err != nil {
		writeError(w, r, h.deps.Log, err)
		return
	}
	writeData(w, http.StatusOK, view)
}

func itoa(n int) string {
	return strconv.Itoa(n)
}

func joinStrings(parts []string, sep string) string {
	return strings.Join(parts, sep)
}
