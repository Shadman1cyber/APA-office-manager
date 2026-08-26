package ai

import (
	"context"
	"fmt"
	"strings"
)

type DocumentInput struct {
	TaskTitle      string
	Topic          string
	AuthorName     string
	ExpectedOutput string
	RawNotes       string
}

type GeneratedDocument struct {
	Title   string `json:"title"`
	Summary string `json:"summary"`
	Body    string `json:"body_markdown"`
}

type DocumentationAgent interface {
	GenerateDocument(ctx context.Context, input DocumentInput) (GeneratedDocument, error)
}

func NewDocumentationAgent(p LLMProvider) DocumentationAgent {
	if _, ok := p.(*MockProvider); ok {
		return &mockDocumentationAgent{}
	}
	return &llmDocumentationAgent{provider: p}
}

const documentSchema = `{"title":"عنوان کوتاه سند","summary":"یک پاراگراف خلاصه اجرایی","body_markdown":"متن کامل سند به صورت مارک‌داون با بخش‌های: خلاصه، شرح اقدامات (فهرست)، نتایج و مستندات"}`

type llmDocumentationAgent struct {
	provider LLMProvider
}

func (l *llmDocumentationAgent) GenerateDocument(ctx context.Context, in DocumentInput) (GeneratedDocument, error) {
	var result GeneratedDocument
	err := l.provider.GenerateStructured(ctx, StructuredRequest{
		Instruction: fmt.Sprintf(
			"این یادداشت کارمند را به یک سند رسمی و حرفه‌ای فارسی تبدیل کن.\nوظیفه: %q\nموضوع: %q\nخروجی مورد انتظار: %q\nانجام‌دهنده: %s\nیادداشت خام:\n%s",
			in.TaskTitle, in.Topic, in.ExpectedOutput, in.AuthorName, in.RawNotes,
		),
		SchemaHint:  documentSchema,
		Temperature: 0.3,
	}, &result)
	if err != nil {
		return GeneratedDocument{}, err
	}
	result.Title = strings.TrimSpace(result.Title)
	result.Body = strings.TrimSpace(result.Body)
	if result.Body == "" {
		return GeneratedDocument{}, fmt.Errorf("%w: empty document body", ErrInvalidLLMOutput)
	}
	if result.Title == "" {
		result.Title = firstNonEmpty(in.TaskTitle, "سند انجام کار")
	}
	return result, nil
}

type mockDocumentationAgent struct{}

func (m *mockDocumentationAgent) GenerateDocument(ctx context.Context, in DocumentInput) (GeneratedDocument, error) {
	if err := ctx.Err(); err != nil {
		return GeneratedDocument{}, err
	}
	title := fmt.Sprintf("سند انجام %s", firstNonEmpty(in.TaskTitle, "کار"))
	subject := firstNonEmpty(in.Topic, "درخواست مدیر")

	var actions strings.Builder
	for _, line := range strings.Split(strings.ReplaceAll(in.RawNotes, "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		actions.WriteString("- ")
		actions.WriteString(line)
		actions.WriteString("\n")
	}

	body := fmt.Sprintf(
		"# %s\n\n## خلاصه\n%s در چارچوب موضوع «%s» فعالیت فوق را انجام داد و نتایج آن در این سند ثبت می‌شود.\n\n"+
			"## شرح اقدامات\n%s\n## نتایج\nبر اساس یادداشت انجام‌دهنده، خروجی کار مطابق «%s» تکمیل گردید.\n\n"+
			"## مستندات\nاین سند به‌صورت خودکار توسط دستیار APA از یادداشت‌های %s تولید شده است.\n",
		title, in.AuthorName, subject, actions.String(),
		firstNonEmpty(in.ExpectedOutput, subject), in.AuthorName,
	)

	summary := fmt.Sprintf("%s وظیفهٔ «%s» را در موضوع «%s» انجام داد.", in.AuthorName, firstNonEmpty(in.TaskTitle, "محول‌شده"), subject)

	return GeneratedDocument{Title: title, Summary: summary, Body: body}, nil
}
