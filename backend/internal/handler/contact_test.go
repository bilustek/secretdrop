package handler_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bilustek/secretdrop/internal/handler"
)

// stubSender is a fake email.Sender for testing.
type stubSender struct {
	called  bool
	to      string
	subject string
	html    string
	err     error
}

func (s *stubSender) Send(_ context.Context, to, subject, html string) error {
	s.called = true
	s.to = to
	s.subject = subject
	s.html = html

	return s.err
}

func TestContactHandler_Success(t *testing.T) {
	t.Parallel()

	sender := &stubSender{}
	h := handler.NewContactHandler(sender)

	body := strings.NewReader(`{"name":"Alice","email":"alice@example.com","message":"Hello!"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/contact", body)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusOK)
	}

	if !sender.called {
		t.Error("expected sender to be called")
	}

	if !strings.Contains(sender.subject, "Alice") {
		t.Errorf("subject = %q; want it to contain 'Alice'", sender.subject)
	}
}

func TestContactHandler_InvalidJSON(t *testing.T) {
	t.Parallel()

	sender := &stubSender{}
	h := handler.NewContactHandler(sender)

	body := strings.NewReader(`not json`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/contact", body)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestContactHandler_MissingFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
	}{
		{"missing name", `{"name":"","email":"a@b.com","message":"hello"}`},
		{"missing email", `{"name":"Alice","email":"","message":"hello"}`},
		{"missing message", `{"name":"Alice","email":"a@b.com","message":""}`},
		{"all empty", `{"name":"","email":"","message":""}`},
		{"whitespace only", `{"name":"  ","email":"  ","message":"  "}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			sender := &stubSender{}
			h := handler.NewContactHandler(sender)

			body := strings.NewReader(tt.body)
			req := httptest.NewRequest(http.MethodPost, "/api/v1/contact", body)
			rec := httptest.NewRecorder()

			h.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Errorf("status = %d; want %d", rec.Code, http.StatusBadRequest)
			}
		})
	}
}

func TestContactHandler_InvalidEmail(t *testing.T) {
	t.Parallel()

	sender := &stubSender{}
	h := handler.NewContactHandler(sender)

	body := strings.NewReader(`{"name":"Alice","email":"not-an-email","message":"hello"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/contact", body)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestContactHandler_SendError(t *testing.T) {
	t.Parallel()

	sender := &stubSender{err: fmt.Errorf("send failed")}
	h := handler.NewContactHandler(sender)

	body := strings.NewReader(`{"name":"Alice","email":"alice@example.com","message":"hello"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/contact", body)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusInternalServerError)
	}
}
