package ai

import (
	"context"
	"errors"
)

var (
	ErrInvalidLLMOutput = errors.New("invalid llm output")
	ErrSmallTalk        = errors.New("message is small talk, not a work request")
)

type GenerateRequest struct {
	System      string
	User        string
	Temperature float64
}

type GenerateResponse struct {
	Text  string
	Model string
}

type StructuredRequest struct {
	System      string
	Instruction string
	SchemaHint  string
	Temperature float64
}

type LLMProvider interface {
	Generate(ctx context.Context, request GenerateRequest) (GenerateResponse, error)
	GenerateStructured(ctx context.Context, request StructuredRequest, output any) error
}
