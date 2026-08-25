package domain

import (
	"errors"
	"fmt"
)

var (
	ErrNotFound         = errors.New("resource not found")
	ErrUnauthorized     = errors.New("unauthorized")
	ErrForbidden        = errors.New("forbidden")
	ErrInvalidState     = errors.New("invalid state")
	ErrApprovalRequired = errors.New("approval required")
	ErrInsufficientData = errors.New("insufficient organizational data")
	ErrEmailTaken       = errors.New("این ایمیل قبلاً ثبت شده است")
)

type ValidationError struct {
	Fields map[string]string
}

func (e *ValidationError) Error() string {
	for field, msg := range e.Fields {
		return fmt.Sprintf("%s: %s", field, msg)
	}
	return "validation failed"
}

func Invalid(field, msg string) error {
	return &ValidationError{Fields: map[string]string{field: msg}}
}
