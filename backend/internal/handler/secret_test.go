package handler_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bilusteknoloji/secretdrop/internal/email/noop"
	"github.com/bilusteknoloji/secretdrop/internal/handler"
	"github.com/bilusteknoloji/secretdrop/internal/model"
	"github.com/bilusteknoloji/secretdrop/internal/repository/sqlite"
	"github.com/bilusteknoloji/secretdrop/internal/service"
)

func newTestHandler(t *testing.T) (*handler.SecretHandler, *http.ServeMux) {
	t.Helper()

	repo, err := sqlite.New(":memory:")
	if err != nil {
		t.Fatalf("sqlite.New() error = %v", err)
	}

	t.Cleanup(func() { repo.Close() })

	sender := noop.New()

	svc, err := service.New(
		repo, sender,
		service.WithBaseURL("http://localhost:3000"),
		service.WithFromEmail("noreply@test.com"),
		service.WithExpiry(10*time.Minute),
	)
	if err != nil {
		t.Fatalf("service.New() error = %v", err)
	}

	h := handler.NewSecretHandler(svc)

	mux := http.NewServeMux()
	h.Register(mux)

	return h, mux
}

func TestHealthz(t *testing.T) {
	t.Parallel()

	_, mux := newTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusOK)
	}

	var resp map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal error = %v", err)
	}

	if resp["status"] != "ok" {
		t.Errorf("status = %q; want %q", resp["status"], "ok")
	}

	if resp["version"] == "" {
		t.Error("version should be present in healthz response")
	}
}

func TestCreateSecret(t *testing.T) {
	t.Parallel()

	_, mux := newTestHandler(t)

	body := `{"text":"API_KEY=secret","to":["test@example.com"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/secrets", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("status = %d; want %d, body: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	var resp model.CreateResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal error = %v", err)
	}

	if resp.ID == "" {
		t.Error("response should have batch ID")
	}

	if len(resp.Recipients) != 1 {
		t.Errorf("recipients = %d; want 1", len(resp.Recipients))
	}
}

func TestCreateAndRevealFullFlow(t *testing.T) {
	t.Parallel()

	_, mux := newTestHandler(t)

	createBody := `{"text":"DB_PASS=secret123","to":["flow@example.com"]}`
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/secrets", strings.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()

	mux.ServeHTTP(createRec, createReq)

	if createRec.Code != http.StatusCreated {
		t.Fatalf("create status = %d; want %d", createRec.Code, http.StatusCreated)
	}

	var createResp model.CreateResponse
	if err := json.Unmarshal(createRec.Body.Bytes(), &createResp); err != nil {
		t.Fatalf("unmarshal create error = %v", err)
	}

	link := createResp.Recipients[0].Link
	parts := strings.SplitN(link, "#", 2)
	tokenPart := parts[0]
	token := tokenPart[strings.LastIndex(tokenPart, "/")+1:]
	encodedKey := parts[1]

	revealBody, _ := json.Marshal(model.RevealRequest{
		Email: "flow@example.com",
		Key:   encodedKey,
	})

	revealReq := httptest.NewRequest(http.MethodPost, "/api/v1/secrets/"+token+"/reveal", bytes.NewReader(revealBody))
	revealReq.Header.Set("Content-Type", "application/json")
	revealRec := httptest.NewRecorder()

	mux.ServeHTTP(revealRec, revealReq)

	if revealRec.Code != http.StatusOK {
		t.Fatalf("reveal status = %d; want %d, body: %s", revealRec.Code, http.StatusOK, revealRec.Body.String())
	}

	var revealResp model.RevealResponse
	if err := json.Unmarshal(revealRec.Body.Bytes(), &revealResp); err != nil {
		t.Fatalf("unmarshal reveal error = %v", err)
	}

	if revealResp.Text != "DB_PASS=secret123" {
		t.Errorf("text = %q; want %q", revealResp.Text, "DB_PASS=secret123")
	}

	revealReq2 := httptest.NewRequest(http.MethodPost, "/api/v1/secrets/"+token+"/reveal", bytes.NewReader(revealBody))
	revealReq2.Header.Set("Content-Type", "application/json")
	revealRec2 := httptest.NewRecorder()

	mux.ServeHTTP(revealRec2, revealReq2)

	if revealRec2.Code != http.StatusNotFound {
		t.Errorf("second reveal status = %d; want %d", revealRec2.Code, http.StatusNotFound)
	}
}

func TestCreateInvalidJSON(t *testing.T) {
	t.Parallel()

	_, mux := newTestHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/secrets", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestCreateValidationErrors(t *testing.T) {
	t.Parallel()

	_, mux := newTestHandler(t)

	body := `{"text":"","to":["a@b.com"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/secrets", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusUnprocessableEntity)
	}
}
