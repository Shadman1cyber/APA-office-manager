package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/apa/backend/internal/ai"
	"github.com/apa/backend/internal/api/middleware"
	"github.com/apa/backend/internal/application"
	"github.com/apa/backend/internal/application/chat"
	documentsvc "github.com/apa/backend/internal/application/document"
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
	Skills    application.SkillRepository
	Bus       *application.Bus
	Log       *slog.Logger
	Ping      func(ctx context.Context) error
	Workflows *workflowsvc.Service
	Tasks     *tasksvc.Service
	Questions *questionsvc.Service
	Knowledge *knowledgesvc.Service
	Documents *documentsvc.Service
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
	case errors.Is(err, domain.ErrAlreadyExists) || errors.Is(err, domain.ErrEmailTaken):
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
	msg := err.Error()

	persianFallbacks := map[string]string{
		"internal_error":    "خطای غیرمنتظره‌ای پیش آمد؛ لطفاً دوباره تلاش کنید.",
		"not_found":         "مورد درخواستی پیدا نشد.",
		"forbidden":         "اجازهٔ انجام این کار را ندارید.",
		"unauthorized":      "ابتدا وارد حساب خود شوید.",
		"invalid_state":     "این عملیات در وضعیت فعلی ممکن نیست.",
		"insufficient_data": "اطلاعات کافی برای انجام این کار وجود ندارد.",
		"ai_output_invalid": "پاسخ هوش مصنوعی قابل استفاده نبود؛ لطفاً دوباره تلاش کنید.",
		"conflict":          "این مورد قبلاً ثبت شده است.",
	}

	if idx := strings.Index(msg, ": "); idx > 0 && !hasPersian(msg[:idx]) {
		msg = msg[idx+2:]
	}
	if hasPersian(msg) {
		return msg
	}
	if fb, ok := persianFallbacks[code]; ok {
		return fb
	}
	return msg
}

func hasPersian(s string) bool {
	for _, r := range s {
		if r >= 0x0600 && r <= 0x06FF {
			return true
		}
	}
	return false
}
