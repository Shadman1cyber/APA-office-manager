package api

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/apa/backend/internal/api/handlers"
	appMiddleware "github.com/apa/backend/internal/api/middleware"
	"github.com/apa/backend/internal/config"
)

func NewRouter(cfg config.Config, logger *slog.Logger, h *handlers.Handlers, tokens *appMiddleware.TokenManager) http.Handler {
	r := chi.NewRouter()

	r.Use(chiMiddleware.RequestID)
	r.Use(chiMiddleware.RealIP)
	r.Use(appMiddleware.RequestLogger(logger))
	r.Use(appMiddleware.CORS(cfg.FrontendOrigin))
	r.Use(appMiddleware.Recovery(logger))

	r.Get("/healthz", h.Healthz)
	r.Post("/api/v1/auth/login", h.Login)
	r.Post("/api/v1/auth/register", h.Register)

	r.Group(func(authed chi.Router) {
		authed.Use(appMiddleware.Auth(tokens))
		authed.Route("/api/v1", func(v1 chi.Router) {
			v1.Get("/auth/me", h.Me)

			v1.Route("/workflows", func(wf chi.Router) {
wf.Get("/", h.ListWorkflows)
			wf.Post("/", h.CreateWorkflow)
			wf.Post("/manual", h.CreateManualWorkflow)
			wf.Get("/{id}", h.GetWorkflow)
			wf.Post("/{id}/approve", h.ApproveWorkflow)
			wf.Post("/{id}/reject", h.RejectWorkflow)
			wf.Delete("/{id}", h.DeleteWorkflow)
			})

			v1.Route("/tasks", func(t chi.Router) {
				t.Get("/", h.ListTasks)
				t.Get("/{id}", h.GetTask)
				t.Patch("/{id}", h.PatchTask)
				t.Post("/{id}/assign", h.AssignTask)
				t.Patch("/{id}/deadline", h.SetTaskDeadline)
			})

			v1.Route("/questions", func(q chi.Router) {
				q.Get("/", h.ListQuestions)
				q.Post("/{id}/answer", h.AnswerQuestion)
			})

			v1.Route("/skills", func(sk chi.Router) {
				sk.Get("/", h.ListSkills)
				sk.Post("/", h.CreateSkill)
			})

			v1.Route("/employees", func(e chi.Router) {
				e.Get("/", h.ListEmployees)
				e.Post("/", h.CreateEmployee)
				e.Patch("/{id}", h.UpdateEmployee)
			})

			v1.Route("/knowledge", func(k chi.Router) {
				k.Get("/", h.KnowledgeOverview)
				k.Get("/people", h.ListPeople)
				k.Get("/facts", h.ListFacts)
				k.Post("/", h.AddFact)
			})

			v1.Get("/ai/chat/history/days", h.ChatHistoryDays)
			v1.Get("/ai/chat/history", h.ChatHistory)
			v1.Route("/documents", func(dc chi.Router) {
				dc.Get("/", h.ListDocuments)
				dc.Post("/", h.CreateDocument)
				dc.Get("/{id}", h.GetDocument)
			})

			v1.Post("/ai/chat", h.Chat)
			v1.Get("/events", h.ListEvents)

			v1.NotFound(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{"error":{"code":"not_found","message":"آدرس درخواستی پیدا نشد."}}`))
			})
		})
	})

	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":{"code":"not_found","message":"آدرس درخواستی پیدا نشد."}}`))
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		_, _ = w.Write([]byte(`{"error":{"code":"method_not_allowed","message":"این متد روی این آدرس پشتیبانی نمی‌شود."}}`))
	})

	return r
}
