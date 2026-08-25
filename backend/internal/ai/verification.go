package ai

import (
	"context"
	"fmt"
	"strings"
)

type VerificationAgent interface {
	VerifyCompletion(ctx context.Context, taskTitle string, completionNotes string) (VerificationResult, error)
}

func NewVerificationAgent(p LLMProvider) VerificationAgent {
	if _, ok := p.(*MockProvider); ok {
		return &mockVerificationAgent{}
	}
	return &llmVerificationAgent{provider: p}
}

type mockVerificationAgent struct{}

func (m *mockVerificationAgent) VerifyCompletion(ctx context.Context, taskTitle string, notes string) (VerificationResult, error) {
	if err := ctx.Err(); err != nil {
		return VerificationResult{}, err
	}
	trimmed := strings.TrimSpace(notes)
	lower := strings.ToLower(trimmed)

	placeholderWords := []string{"todo", "tbd", "placeholder", "lorem ipsum"}
	for _, w := range placeholderWords {
		if strings.Contains(lower, w) {
			return VerificationResult{
				Passed:     false,
				Feedback:   fmt.Sprintf("یادداشت تکمیل برای «%s» شامل جای‌نگهدار (%s) است. لطفاً نتیجهٔ واقعی ارائه دهید.", taskTitle, w),
				Confidence: 0.9,
			}, nil
		}
	}
	if len(trimmed) < 30 {
		return VerificationResult{
			Passed:     false,
			Feedback:   fmt.Sprintf("یادداشت تکمیل برای «%s» برای بررسی بسیار کوتاه است. توضیح دهید چه چیزی تولید شده است.", taskTitle),
			Confidence: 0.8,
		}, nil
	}
	return VerificationResult{
		Passed:     true,
		Feedback:   "یادداشت‌های تکمیل مشخص هستند و با خروجی مورد انتظار سازگارند.",
		Confidence: 0.85,
	}, nil
}

const verificationSchema = `{"passed":true,"feedback":"specific actionable feedback","confidence":0.85}`

type llmVerificationAgent struct {
	provider LLMProvider
}

func (l *llmVerificationAgent) VerifyCompletion(ctx context.Context, taskTitle string, notes string) (VerificationResult, error) {
	var result VerificationResult
	err := l.provider.GenerateStructured(ctx, StructuredRequest{
		Instruction: fmt.Sprintf("Verify whether these completion notes satisfy the task %q.\nNotes: %q", taskTitle, notes),
		SchemaHint:  verificationSchema,
	}, &result)
	if err != nil {
		return VerificationResult{}, err
	}
	result.Confidence = clamp01(result.Confidence)
	if result.Feedback == "" {
		if result.Passed {
			result.Feedback = "بررسی شد و تأیید گردید."
		} else {
			result.Feedback = "امکان بررسی نیست؛ نیازمند بازبینی انسانی."
		}
	}
	return result, nil
}
