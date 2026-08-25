package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/apa/backend/internal/ai"
	"github.com/apa/backend/internal/api/middleware"
	"github.com/apa/backend/internal/application"
	"github.com/apa/backend/internal/application/chat"
	"github.com/apa/backend/internal/application/knowledge"
	"github.com/apa/backend/internal/application/question"
	"github.com/apa/backend/internal/application/task"
	"github.com/apa/backend/internal/application/workflow"
	"github.com/apa/backend/internal/domain"
)

type Deps struct {
	Tokens    *middleware.TokenManager
	Users     application.UserRepository
	Orgs      application.OrganizationRepository
	Bus       *application.Bus
	Log       *slog.Logger
	Ping      func(ctx context.Context) error
	Workflows *workflowsvc.Service
	Tasks     *tasksvc.Service
	Questions *questionsvc.Service
	Knowledge *knowledgesvc.Service
	Chat      *chatsvc.Service
}

type Handlers struct {
	deps Deps
}

func New(deps Deps) *Handlers {
	return &Handlers{deps: deps}
}

func (h *Handlers) identity(r *http.Request) (*middleware.Identity, bool) {
	return middleware.IdentityFrom(r.Context())
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		slog.Error("encode response failed", slog.Any("error", err))
	}
}

func writeData(w http.ResponseWriter, status int, data any) {
	writeJSON(w, status, map[string]any{"data": data})
}

type errorBody struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
}

func writeError(w http.ResponseWriter, r *http.Request, log *slog.Logger, err error) {
	var code string
	var status int

	switch {
	case errors.Is(err, domain.ErrNotFound):
		code, status = "not_found", http.StatusNotFound
	case errors.Is(err, domain.ErrUnauthorized):
		code, status = "unauthorized", http.StatusUnauthorized
	case errors.Is(err, domain.ErrForbidden):
		code, status = "forbidden", http.StatusForbidden
	case errors.Is(err, domain.ErrInvalidState):
		code, status = "invalid_state", http.StatusConflict
	case errors.Is(err, domain.ErrApprovalRequired):
		code, status = "approval_required", http.StatusAccepted
	case errors.Is(err, ai.ErrSmallTalk):
		code, status = "not_a_task", http.StatusUnprocessableEntity
	case errors.Is(err, domain.ErrEmailTaken):
		code, status = "conflict", http.StatusConflict
	case errors.Is(err, domain.ErrInsufficientData):
		code, status = "insufficient_data", http.StatusUnprocessableEntity
	case errors.Is(err, ai.ErrInvalidLLMOutput):
		code, status = "ai_output_invalid", http.StatusBadGateway
	default:
		var validation *domain.ValidationError
		if errors.As(err, &validation) {
			writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
				"error": errorBody{Code: "validation_error", Message: err.Error(), Fields: validation.Fields},
			})
			return
		}
		code, status = "internal_error", http.StatusInternalServerError
		log.ErrorContext(r.Context(), "internal error",
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Any("error", err))
	}

	message := publicMessage(code, err)
	writeJSON(w, status, map[string]any{
		"error": errorBody{Code: code, Message: message},
	})
}

func publicMessage(code string, err error) string {
	switch code {
	case "internal_error":
		return "something went wrong; please retry"
	case "ai_output_invalid":
		return "the AI service returned an unusable response"
	default:
		return err.Error()
	}
}
