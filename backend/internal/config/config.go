package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	Env            string
	DatabaseURL    string
	ServerPort     int
	JWTSecret      string
	LLMProvider    string
	LLMAPIKey      string
	LLMModel       string
	LLMBaseURL     string
	FrontendOrigin string
	LogLevel       string
	SeedDemo       bool
}

func Load() (Config, error) {
	cfg := Config{
		Env:            getEnv("ENV", "development"),
		DatabaseURL:    getEnv("DATABASE_URL", "postgres://apa:apa@localhost:5432/apa?sslmode=disable"),
		ServerPort:     8080,
		JWTSecret:      getEnv("JWT_SECRET", ""),
		LLMProvider:    getEnv("LLM_PROVIDER", "mock"),
		LLMAPIKey:      getEnv("LLM_API_KEY", ""),
		LLMModel:       getEnv("LLM_MODEL", "gpt-4o-mini"),
		LLMBaseURL:     getEnv("LLM_BASE_URL", "https://api.openai.com/v1"),
		FrontendOrigin: getEnv("FRONTEND_ORIGIN", "http://localhost:3000"),
		LogLevel:       getEnv("LOG_LEVEL", "info"),
		SeedDemo:       getEnv("SEED_DEMO", "false") == "true",
	}

	if raw := os.Getenv("SERVER_PORT"); raw != "" {
		port, err := strconv.Atoi(raw)
		if err != nil {
			return cfg, fmt.Errorf("invalid SERVER_PORT %q: %w", raw, err)
		}
		cfg.ServerPort = port
	}

	if cfg.JWTSecret == "" {
		if cfg.Env == "production" {
			return cfg, fmt.Errorf("JWT_SECRET is required in production")
		}
		cfg.JWTSecret = "dev-insecure-secret-change-me"
	}

	switch cfg.LLMProvider {
	case "mock", "openai":
	default:
		return cfg, fmt.Errorf("invalid LLM_PROVIDER %q: must be mock or openai", cfg.LLMProvider)
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
