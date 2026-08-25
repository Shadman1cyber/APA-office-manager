package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/apa/backend/internal/infrastructure/llm"
)

type OpenAIProvider struct {
	client *llm.Client
	model  string
}

func NewOpenAIProvider(client *llm.Client, model string) *OpenAIProvider {
	return &OpenAIProvider{client: client, model: model}
}

func (o *OpenAIProvider) Generate(ctx context.Context, request GenerateRequest) (GenerateResponse, error) {
	messages := []llm.Message{}
	if request.System != "" {
		messages = append(messages, llm.Message{Role: "system", Content: request.System})
	}
	messages = append(messages, llm.Message{Role: "user", Content: request.User})

	text, err := o.client.ChatCompletion(ctx, messages, request.Temperature, false)
	if err != nil {
		return GenerateResponse{}, err
	}
	return GenerateResponse{Text: text, Model: o.model}, nil
}

func (o *OpenAIProvider) GenerateStructured(ctx context.Context, request StructuredRequest, output any) error {
	system := request.System
	if system == "" {
		system = "You are a precise planning assistant inside an enterprise workflow product."
	}
	system += " Respond with ONLY a single minified JSON object, no prose, no markdown." +
		" Write every human-readable text value (titles, descriptions, questions, reasons, evidence, feedback) in Persian (Farsi)." +
		" Keep JSON keys and enum values (like create_report) in English."
	if request.SchemaHint != "" {
		system += " The JSON must match exactly this shape: " + request.SchemaHint
	}

	messages := []llm.Message{
		{Role: "system", Content: system},
		{Role: "user", Content: request.Instruction},
	}

	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(400 * time.Millisecond):
			}
		}
		text, err := o.client.ChatCompletion(ctx, messages, request.Temperature, true)
		if err != nil {
			lastErr = err
			continue
		}

		cleaned := extractJSON(text)
		if cleaned == "" {
			lastErr = fmt.Errorf("%w: empty response", ErrInvalidLLMOutput)
			continue
		}
		if err := json.Unmarshal([]byte(cleaned), output); err != nil {
			lastErr = fmt.Errorf("%w: %v (raw: %.200s)", ErrInvalidLLMOutput, err, text)
			continue
		}
		return nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("%w: all attempts failed", ErrInvalidLLMOutput)
	}
	return lastErr
}

func extractJSON(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start >= 0 && end > start {
		return raw[start : end+1]
	}
	return strings.TrimSpace(raw)
}
