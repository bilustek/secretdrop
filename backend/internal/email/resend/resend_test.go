package resend

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/bilustek/secretdrop/internal/email"

	resendapi "github.com/resend/resend-go/v2"
)

func mustParseURL(t *testing.T, rawURL string) *url.URL {
	t.Helper()

	u, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse URL %q: %v", rawURL, err)
	}

	return u
}

func TestSenderImplementsEmailSender(t *testing.T) {
	t.Parallel()

	var _ email.Sender = (*Sender)(nil)
}

func TestNew(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		apiKey  string
		opts    []Option
		wantErr string
	}{
		{
			name:    "empty API key returns error",
			apiKey:  "",
			wantErr: "API key cannot be empty",
		},
		{
			name:   "valid API key creates sender",
			apiKey: "re_test_123",
		},
		{
			name:   "with valid from option",
			apiKey: "re_test_123",
			opts:   []Option{WithFrom("noreply@example.com")},
		},
		{
			name:   "with valid reply-to option",
			apiKey: "re_test_123",
			opts:   []Option{WithReplyTo("support@example.com")},
		},
		{
			name:   "with all options",
			apiKey: "re_test_123",
			opts: []Option{
				WithFrom("noreply@example.com"),
				WithReplyTo("support@example.com"),
			},
		},
		{
			name:    "with empty from option returns error",
			apiKey:  "re_test_123",
			opts:    []Option{WithFrom("")},
			wantErr: "apply option: from email cannot be empty",
		},
		{
			name:    "with empty reply-to option returns error",
			apiKey:  "re_test_123",
			opts:    []Option{WithReplyTo("")},
			wantErr: "apply option: reply-to email cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			sender, err := New(tt.apiKey, tt.opts...)

			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("New() error = nil; want %q", tt.wantErr)
				}

				if err.Error() != tt.wantErr {
					t.Fatalf("New() error = %q; want %q", err.Error(), tt.wantErr)
				}

				if sender != nil {
					t.Fatal("New() sender should be nil on error")
				}

				return
			}

			if err != nil {
				t.Fatalf("New() error = %v; want nil", err)
			}

			if sender == nil {
				t.Fatal("New() sender = nil; want non-nil")
			}

			if sender.client == nil {
				t.Fatal("New() sender.client = nil; want non-nil")
			}
		})
	}
}

func TestNewSetsFromAndReplyTo(t *testing.T) {
	t.Parallel()

	from := "sender@example.com"
	replyTo := "reply@example.com"

	sender, err := New("re_test_123", WithFrom(from), WithReplyTo(replyTo))
	if err != nil {
		t.Fatalf("New() error = %v; want nil", err)
	}

	if sender.from != from {
		t.Errorf("sender.from = %q; want %q", sender.from, from)
	}

	if sender.replyTo != replyTo {
		t.Errorf("sender.replyTo = %q; want %q", sender.replyTo, replyTo)
	}
}

func TestWithFrom(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		from    string
		wantErr bool
	}{
		{name: "valid from", from: "test@example.com"},
		{name: "empty from", from: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			opt := WithFrom(tt.from)
			s := &Sender{}
			err := opt(s)

			if tt.wantErr && err == nil {
				t.Fatal("WithFrom() error = nil; want error")
			}

			if !tt.wantErr && err != nil {
				t.Fatalf("WithFrom() error = %v; want nil", err)
			}

			if !tt.wantErr && s.from != tt.from {
				t.Errorf("from = %q; want %q", s.from, tt.from)
			}
		})
	}
}

func TestWithReplyTo(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		replyTo string
		wantErr bool
	}{
		{name: "valid reply-to", replyTo: "reply@example.com"},
		{name: "empty reply-to", replyTo: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			opt := WithReplyTo(tt.replyTo)
			s := &Sender{}
			err := opt(s)

			if tt.wantErr && err == nil {
				t.Fatal("WithReplyTo() error = nil; want error")
			}

			if !tt.wantErr && err != nil {
				t.Fatalf("WithReplyTo() error = %v; want nil", err)
			}

			if !tt.wantErr && s.replyTo != tt.replyTo {
				t.Errorf("replyTo = %q; want %q", s.replyTo, tt.replyTo)
			}
		})
	}
}

func TestSendSuccess(t *testing.T) {
	t.Parallel()

	var receivedReq resendapi.SendEmailRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s; want POST", r.Method)
		}

		if !strings.HasSuffix(r.URL.Path, "/emails") {
			t.Errorf("path = %s; want suffix /emails", r.URL.Path)
		}

		if err := json.NewDecoder(r.Body).Decode(&receivedReq); err != nil {
			t.Errorf("decode request body: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		resp := map[string]string{"id": "email_123"}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Errorf("encode response: %v", err)
		}
	}))
	defer server.Close()

	sender, err := New("re_test_key", WithFrom("from@example.com"), WithReplyTo("reply@example.com"))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	sender.client.BaseURL = mustParseURL(t, server.URL)

	err = sender.Send(context.Background(), "to@example.com", "Test Subject", "<p>Hello</p>")
	if err != nil {
		t.Fatalf("Send() error = %v; want nil", err)
	}

	if receivedReq.From != "from@example.com" {
		t.Errorf("request From = %q; want %q", receivedReq.From, "from@example.com")
	}

	if len(receivedReq.To) != 1 || receivedReq.To[0] != "to@example.com" {
		t.Errorf("request To = %v; want [to@example.com]", receivedReq.To)
	}

	if receivedReq.Subject != "Test Subject" {
		t.Errorf("request Subject = %q; want %q", receivedReq.Subject, "Test Subject")
	}

	if receivedReq.Html != "<p>Hello</p>" {
		t.Errorf("request Html = %q; want %q", receivedReq.Html, "<p>Hello</p>")
	}

	if receivedReq.ReplyTo != "reply@example.com" {
		t.Errorf("request ReplyTo = %q; want %q", receivedReq.ReplyTo, "reply@example.com")
	}
}

func TestSendAPIError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)

		resp := map[string]string{"message": "internal server error"}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Errorf("encode response: %v", err)
		}
	}))
	defer server.Close()

	sender, err := New("re_test_key", WithFrom("from@example.com"))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	sender.client.BaseURL = mustParseURL(t, server.URL)

	err = sender.Send(context.Background(), "to@example.com", "Test", "<p>Hi</p>")
	if err == nil {
		t.Fatal("Send() error = nil; want error")
	}

	if !strings.Contains(err.Error(), "send email via resend") {
		t.Errorf("Send() error = %q; want to contain %q", err.Error(), "send email via resend")
	}
}

func TestSendWithoutReplyTo(t *testing.T) {
	t.Parallel()

	var receivedReq resendapi.SendEmailRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&receivedReq); err != nil {
			t.Errorf("decode request body: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		resp := map[string]string{"id": "email_456"}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Errorf("encode response: %v", err)
		}
	}))
	defer server.Close()

	sender, err := New("re_test_key", WithFrom("from@example.com"))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	sender.client.BaseURL = mustParseURL(t, server.URL)

	err = sender.Send(context.Background(), "to@example.com", "Subject", "<p>Body</p>")
	if err != nil {
		t.Fatalf("Send() error = %v; want nil", err)
	}

	if receivedReq.ReplyTo != "" {
		t.Errorf("request ReplyTo = %q; want empty", receivedReq.ReplyTo)
	}
}
