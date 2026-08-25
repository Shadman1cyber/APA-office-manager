package worker

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/apa/backend/internal/repository"
)

type Handler func(ctx context.Context, payload []byte) error

type Worker struct {
	jobs        *repository.Jobs
	handlers    map[string]Handler
	log         *slog.Logger
	interval    time.Duration
	concurrency int

	mu sync.RWMutex
}

func New(jobs *repository.Jobs, log *slog.Logger) *Worker {
	return &Worker{
		jobs:        jobs,
		handlers:    make(map[string]Handler),
		log:         log,
		interval:    750 * time.Millisecond,
		concurrency: 4,
	}
}

func (w *Worker) Register(jobType string, handler Handler) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.handlers[jobType] = handler
}

func (w *Worker) Run(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	sem := make(chan struct{}, w.concurrency)
	var wg sync.WaitGroup

	w.log.InfoContext(ctx, "worker started",
		slog.Int("concurrency", w.concurrency),
		slog.Duration("interval", w.interval))

	for {
		select {
		case <-ctx.Done():
			wg.Wait()
			w.log.InfoContext(ctx, "worker stopped")
			return
		case <-ticker.C:
			claimed, err := w.jobs.Claim(ctx, w.concurrency*2)
			if err != nil {
				if ctx.Err() == nil {
					w.log.ErrorContext(ctx, "claim jobs failed", slog.Any("error", err))
				}
				continue
			}
			for _, job := range claimed {
				job := job
				select {
				case sem <- struct{}{}:
					wg.Add(1)
					go func() {
						defer wg.Done()
						defer func() { <-sem }()
						w.process(ctx, job)
					}()
				case <-ctx.Done():
					wg.Wait()
					return
				}
			}
		}
	}
}

func (w *Worker) process(ctx context.Context, job repository.Job) {
	jobCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	w.mu.RLock()
	handler, ok := w.handlers[job.Type]
	w.mu.RUnlock()

	if !ok {
		w.log.ErrorContext(jobCtx, "no handler registered for job type", slog.String("type", job.Type))
		if err := w.jobs.MarkFailed(jobCtx, job.ID, "no handler registered", false, 0); err != nil {
			w.log.ErrorContext(jobCtx, "mark job failed", slog.Any("error", err))
		}
		return
	}

	err := func() (err error) {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("job panicked: %v", r)
			}
		}()
		return handler(jobCtx, job.Payload)
	}()

	if err == nil {
		if derr := w.jobs.MarkDone(jobCtx, job.ID); derr != nil {
			w.log.ErrorContext(jobCtx, "mark job done failed", slog.Any("error", derr))
		}
		return
	}

	retry := job.Attempts < job.MaxAttempts
	backoff := time.Duration(job.Attempts) * 5 * time.Second
	level := slog.LevelError
	logAttrs := []any{
		slog.Int64("job_id", job.ID),
		slog.String("type", job.Type),
		slog.Any("error", err),
		slog.Bool("will_retry", retry),
	}
	w.log.Log(jobCtx, level, "job failed", logAttrs...)

	if ferr := w.jobs.MarkFailed(jobCtx, job.ID, err.Error(), retry, backoff); ferr != nil {
		w.log.ErrorContext(jobCtx, "mark job failed", slog.Any("error", ferr))
	}
}
