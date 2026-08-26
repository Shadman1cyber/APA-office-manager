package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/apa/backend/internal/domain"
)

var _ = domain.ErrNotFound

type errorLogger interface {
	ErrorContext(ctx context.Context, msg string, args ...any)
}

// Recovery catches panics from downstream handlers and returns a clean Persian
// error envelope instead of an empty 500, so a panic can never end a request
// without an understandable explanation.
func Recovery(log errorLogger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					log.ErrorContext(r.Context(), "panic recovered",
						slog.String("method", r.Method),
						slog.String("path", r.URL.Path),
						slog.Any("panic", rec),
						slog.String("stack", string(debug.Stack())),
					)
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusInternalServerError)
					_, _ = w.Write([]byte(`{"error":{"code":"internal_error","message":"خطای غیرمنتظره‌ای در سرور رخ داد؛ تیم فنی مطلع شد. لطفاً دوباره تلاش کنید."}}`))
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}
