package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"

	"github.com/apa/backend"
	"github.com/apa/backend/internal/ai"
	"github.com/apa/backend/internal/api"
	"github.com/apa/backend/internal/api/handlers"
	"github.com/apa/backend/internal/api/middleware"
	"github.com/apa/backend/internal/application"
	"github.com/apa/backend/internal/application/approval"
	"github.com/apa/backend/internal/application/chat"
	"github.com/apa/backend/internal/application/knowledge"
	"github.com/apa/backend/internal/application/question"
	"github.com/apa/backend/internal/application/task"
	"github.com/apa/backend/internal/application/workflow"
	"github.com/apa/backend/internal/config"
	"github.com/apa/backend/internal/infrastructure/database"
	"github.com/apa/backend/internal/infrastructure/llm"
	"github.com/apa/backend/internal/infrastructure/logging"
	"github.com/apa/backend/internal/repository"
	"github.com/apa/backend/internal/seed"
	"github.com/apa/backend/internal/worker"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	logger := logging.Setup(cfg.LogLevel)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	migrationsFS, err := fs.Sub(backend.MigrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("embed migrations: %w", err)
	}
	if err := database.Migrate(ctx, migrationsFS, cfg.DatabaseURL); err != nil {
		return fmt.Errorf("migrate database: %w", err)
	}
	logger.InfoContext(ctx, "database migrations applied")

	pool, err := database.ConnectPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("connect database: %w", err)
	}
	defer pool.Close()

	if cfg.SeedDemo {
		if err := seed.Run(ctx, seed.Deps{
			Orgs:      repository.NewOrganizations(pool),
			Users:     repository.NewUsers(pool),
			Knowledge: repository.NewKnowledge(pool),
			Log:       logger,
		}); err != nil {
			return fmt.Errorf("seed database: %w", err)
		}
	} else {
		logger.InfoContext(ctx, "demo seeding disabled; starting with a clean organization")
	}

	usersRepo := repository.NewUsers(pool)
	workflowsRepo := repository.NewWorkflows(pool)
	tasksRepo := repository.NewTasks(pool)
	questionsRepo := repository.NewQuestions(pool)
	knowledgeRepo := repository.NewKnowledge(pool)
	approvalsRepo := repository.NewApprovals(pool)
	eventsRepo := repository.NewEvents(pool)
	jobsRepo := repository.NewJobs(pool)
	orgReader := repository.NewOrgReader(usersRepo, knowledgeRepo)

	bus := application.NewBus(eventsRepo, logger)

	var provider ai.LLMProvider
	switch cfg.LLMProvider {
	case "openai":
		provider = ai.NewOpenAIProvider(llm.NewClient(cfg.LLMBaseURL, cfg.LLMAPIKey, cfg.LLMModel), cfg.LLMModel)
	default:
		provider = ai.NewMockProvider()
	}
	logger.InfoContext(ctx, "llm provider selected", slog.String("provider", cfg.LLMProvider))

	mockProvider := ai.NewMockProvider()
	resilientOrchestrator := ai.NewOrchestrator(
		ai.NewResilientIntentAgent(ai.NewIntentAgent(provider), ai.NewIntentAgent(mockProvider), logger),
		ai.NewContextAgent(orgReader),
		ai.NewResilientPlanningAgent(ai.NewPlanningAgent(provider), ai.NewPlanningAgent(mockProvider), logger),
		ai.NewResilientQuestionAgent(ai.NewQuestionAgent(provider), ai.NewQuestionAgent(mockProvider), logger),
		ai.NewResilientAssignmentAgent(ai.NewAssignmentAgent(provider), ai.NewAssignmentAgent(mockProvider), logger),
		ai.NewResilientVerificationAgent(ai.NewVerificationAgent(provider), ai.NewVerificationAgent(mockProvider), logger),
		ai.NewResilientLearningAgent(ai.NewLearningAgent(provider), ai.NewLearningAgent(mockProvider), logger),
		logger,
	)
	orchestrator := resilientOrchestrator

	knowledgeService := knowledgesvc.NewService(knowledgeRepo, usersRepo, bus, logger)
	approvalService := approvalsvc.NewService(approvalsRepo, bus)

	workflowService := workflowsvc.NewService(workflowsvc.Deps{
		Workflows:    workflowsRepo,
		Tasks:        tasksRepo,
		Questions:    questionsRepo,
		Approvals:    approvalService,
		Users:        usersRepo,
		Orchestrator: orchestrator,
		Bus:          bus,
		Log:          logger,
	})
	taskService := tasksvc.NewService(tasksvc.Deps{
		Tasks:        tasksRepo,
		Workflows:    workflowsRepo,
		Users:        usersRepo,
		Knowledge:    knowledgeService,
		Approvals:    approvalService,
		Orchestrator: orchestrator,
		Bus:          bus,
		Jobs:         jobsRepo,
		Log:          logger,
	})
	questionService := questionsvc.NewService(questionsvc.Deps{
		Questions:    questionsRepo,
		Tasks:        tasksRepo,
		Users:        usersRepo,
		Knowledge:    knowledgeService,
		TasksService: taskService,
		Orchestrator: orchestrator,
		Bus:          bus,
		Log:          logger,
	})
	chatService := chatsvc.NewService(workflowService, questionService, repository.NewChat(pool))

	backgroundWorker := worker.New(jobsRepo, logger)
	backgroundWorker.Register(tasksvc.VerifyJobType, func(jobCtx context.Context, payload []byte) error {
		var p struct {
			TaskID string `json:"task_id"`
			OrgID  string `json:"org_id"`
		}
		if err := json.Unmarshal(payload, &p); err != nil {
			return fmt.Errorf("decode job payload: %w", err)
		}
		taskID, err := uuid.Parse(p.TaskID)
		if err != nil {
			return fmt.Errorf("invalid task id in job payload: %w", err)
		}
		orgID, err := uuid.Parse(p.OrgID)
		if err != nil {
			return fmt.Errorf("invalid org id in job payload: %w", err)
		}
		return taskService.VerifyCompleted(jobCtx, orgID, taskID)
	})
	go backgroundWorker.Run(ctx)

	tokens := middleware.NewTokenManager(cfg.JWTSecret)
	apiHandlers := handlers.New(handlers.Deps{
		Tokens:    tokens,
		Users:     usersRepo,
		Orgs:      repository.NewOrganizations(pool),
		Bus:       bus,
		Log:       logger,
		Ping:      pool.Ping,
		Workflows: workflowService,
		Tasks:     taskService,
		Questions: questionService,
		Knowledge: knowledgeService,
		Chat:      chatService,
	})

	router := api.NewRouter(cfg, logger, apiHandlers, tokens)

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.ServerPort),
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.InfoContext(ctx, "server listening",
			slog.Int("port", cfg.ServerPort),
			slog.String("env", cfg.Env))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutdown server: %w", err)
		}
		logger.InfoContext(ctx, "server stopped gracefully")
		return nil
	case err := <-errCh:
		return fmt.Errorf("server error: %w", err)
	}
}
