package model_test

import (
	"testing"

	"github.com/bilusteknoloji/secretdrop/internal/model"
)

func TestAppErrorReturnsMessage(t *testing.T) {
	t.Parallel()

	appErr := &model.AppError{
		Type:       "test_error",
		Message:    "something went wrong",
		StatusCode: 500,
	}

	if appErr.Error() != "something went wrong" {
		t.Errorf("Error() = %q; want %q", appErr.Error(), "something went wrong")
	}
}

func TestAppErrorImplementsErrorInterface(t *testing.T) {
	t.Parallel()

	appErr := &model.AppError{
		Type:       "not_found",
		Message:    "resource not found",
		StatusCode: 404,
	}

	var err error = appErr

	if err.Error() != "resource not found" {
		t.Errorf("Error() = %q; want %q", err.Error(), "resource not found")
	}
}
