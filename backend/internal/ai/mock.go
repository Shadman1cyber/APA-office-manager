package ai

import (
	"context"
	"fmt"
)

type MockProvider struct{}

func NewMockProvider() *MockProvider {
	return &MockProvider{}
}

func (m *MockProvider) Generate(ctx context.Context, request GenerateRequest) (GenerateResponse, error) {
	if err := ctx.Err(); err != nil {
		return GenerateResponse{}, err
	}
	reply := fmt.Sprintf("[mock] processed: %s", request.User)
	return GenerateResponse{Text: reply, Model: "mock"}, nil
}

func (m *MockProvider) GenerateStructured(ctx context.Context, request StructuredRequest, output any) error {
	return fmt.Errorf("%w: mock provider only supports agent-specific deterministic logic", ErrInvalidLLMOutput)
}
