package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/apa/backend/internal/application/chat"
	workflowsvc "github.com/apa/backend/internal/application/workflow"
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
	Intent   string `json:"intent"`
	Deadline string `json:"deadline"`
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
	var deadline *time.Time
	if req.Deadline != "" {
		parsed, perr := time.Parse(time.RFC3339, req.Deadline)
		if perr != nil {
			writeError(w, r, h.deps.Log, domain.Invalid("deadline", "فرمت مهلت نامعتبر است"))
			return
		}
		if parsed.Before(time.Now()) {
			writeError(w, r, h.deps.Log, domain.Invalid("deadline", "مهلت باید در آینده باشد"))
			return
		}
		deadline = &parsed
	}
	view, err := h.deps.Workflows.Create(r.Context(), actor, req.Intent, deadline)
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
		if deadline != nil {
			reply.Text += "\nمهلت انجام همهٔ وظایف: " + deadline.In(time.FixedZone("Asia/Tehran", 3*3600+1800)).Format("2006/01/02 ۱۵:۰۴")
		}
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
		writeError(w, r, h.deps.Log, domain.Invalid("id", "شناسه گردش‌کار نامعتبر است"))
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

type manualTaskRequest struct {
	Title          string   `json:"title"`
	Description    string   `json:"description"`
	Topic          string   `json:"topic"`
	RequiredSkills []string `json:"requiredSkills"`
	Deadline       string   `json:"deadline"`
	AssignedTo     string   `json:"assignedTo"`
}

type createManualWorkflowRequest struct {
	Title    string              `json:"title"`
	Intent   string              `json:"intent"`
	Deadline string              `json:"deadline"`
	Tasks    []manualTaskRequest `json:"tasks"`
}

func (h *Handlers) CreateManualWorkflow(w http.ResponseWriter, r *http.Request) {
	actor, err := h.actorUser(r)
	if err != nil {
		writeError(w, r, h.deps.Log, err)
		return
	}
	if !actor.Role.CanApprove() {
		writeError(w, r, h.deps.Log, fmt.Errorf("%w: تعریف دستی گردش‌کار فقط برای مدیران مجاز است", domain.ErrForbidden))
		return
	}

	var req createManualWorkflowRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, r, h.deps.Log, err)
		return
	}

	var defaultDeadline *time.Time
	if req.Deadline != "" {
		parsed, perr := time.Parse(time.RFC3339, req.Deadline)
		if perr != nil {
			writeError(w, r, h.deps.Log, domain.Invalid("deadline", "فرمت مهلت نامعتبر است"))
			return
		}
		defaultDeadline = &parsed
	}

	in := workflowsvc.ManualWorkflowInput{
		Title:    req.Title,
		Intent:   req.Intent,
		Deadline: defaultDeadline,
	}

	for i, mt := range req.Tasks {
		mi := workflowsvc.ManualTaskInput{
			Title:          mt.Title,
			Description:    mt.Description,
			Topic:          mt.Topic,
			RequiredSkills: normalizeSkills(mt.RequiredSkills),
		}
		if verr := h.validateSkillsAgainstCatalog(r, actor.OrgID, mi.RequiredSkills); verr != nil {
			writeError(w, r, h.deps.Log, domain.Invalid(fmt.Sprintf("tasks[%d].requiredSkills", i), "برای وظیفهٔ «"+mt.Title+"» "+verr.Error()))
			return
		}
		if mt.Deadline != "" {
			parsed, perr := time.Parse(time.RFC3339, mt.Deadline)
			if perr != nil {
				writeError(w, r, h.deps.Log, domain.Invalid(fmt.Sprintf("tasks[%d].deadline", i), "فرمت مهلت نامعتبر است"))
				return
			}
			mi.Deadline = &parsed
		} else {
			mi.Deadline = defaultDeadline
		}
		if mt.AssignedTo != "" {
			parsed, perr := parseUUID(mt.AssignedTo)
			if perr != nil {
				writeError(w, r, h.deps.Log, domain.Invalid(fmt.Sprintf("tasks[%d].assignedTo", i), "شناسه مسئول نامعتبر است"))
				return
			}
			assignee, aerr := h.deps.Users.Get(r.Context(), parsed)
			if aerr != nil || assignee.OrgID != actor.OrgID {
				writeError(w, r, h.deps.Log, domain.Invalid(fmt.Sprintf("tasks[%d].assignedTo", i), "مسئول باید عضو همین سازمان باشد"))
				return
			}
			mi.AssignedTo = &parsed
		}
		in.Tasks = append(in.Tasks, mi)
	}

	view, err := h.deps.Workflows.CreateManual(r.Context(), actor, in)
	if err != nil {
		writeError(w, r, h.deps.Log, err)
		return
	}
	writeData(w, http.StatusCreated, map[string]any{
		"message":  fmt.Sprintf("گردش‌کار «%s» با %d وظیفه ساخته شد.", view.Workflow.Title, len(view.Tasks)),
		"workflow": view,
	})
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
		writeError(w, r, h.deps.Log, domain.Invalid("id", "شناسه گردش‌کار نامعتبر است"))
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
		writeError(w, r, h.deps.Log, domain.Invalid("id", "شناسه گردش‌کار نامعتبر است"))
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

func (h *Handlers) DeleteWorkflow(w http.ResponseWriter, r *http.Request) {
	actor, err := h.actorUser(r)
	if err != nil {
		writeError(w, r, h.deps.Log, err)
		return
	}
	if !actor.Role.CanApprove() {
		writeError(w, r, h.deps.Log, fmt.Errorf("%w: تنها مدیر می‌تواند گردش‌کار حذف کند", domain.ErrForbidden))
		return
	}
	wfID, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, r, h.deps.Log, domain.Invalid("id", "شناسه گردش‌کار نامعتبر است"))
		return
	}
	if err := h.deps.Workflows.Delete(r.Context(), actor, wfID); err != nil {
		writeError(w, r, h.deps.Log, err)
		return
	}
	writeData(w, http.StatusOK, map[string]any{
		"success":  true,
		"message":  "گردش‌کار حذف شد.",
	})
}

func itoa(n int) string {
	return strconv.Itoa(n)
}

func joinStrings(parts []string, sep string) string {
	return strings.Join(parts, sep)
}
