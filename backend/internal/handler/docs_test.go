package handler_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bilusteknoloji/secretdrop/internal/handler"
)

func TestOpenAPISpec(t *testing.T) {
	specContent := []byte("openapi: 3.0.0\ninfo:\n  title: Test\n")
	handler.SetOpenAPISpec(specContent)

	mux := http.NewServeMux()
	handler.RegisterDocs(mux, nil)

	req := httptest.NewRequest(http.MethodGet, "/docs/openapi.yaml", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusOK)
	}

	ct := rec.Header().Get("Content-Type")
	if ct != "application/yaml" {
		t.Errorf("Content-Type = %q; want %q", ct, "application/yaml")
	}

	if rec.Body.String() != string(specContent) {
		t.Errorf("body = %q; want spec content", rec.Body.String())
	}
}

func TestOpenAPISpecHasCORSHeader(t *testing.T) {
	handler.SetOpenAPISpec([]byte("openapi: 3.0.0"))

	mux := http.NewServeMux()
	handler.RegisterDocs(mux, nil)

	req := httptest.NewRequest(http.MethodGet, "/docs/openapi.yaml", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	cors := rec.Header().Get("Access-Control-Allow-Origin")
	if cors != "*" {
		t.Errorf("Access-Control-Allow-Origin = %q; want %q", cors, "*")
	}
}

func TestRegisterDocs_WithProtect(t *testing.T) {
	specContent := []byte("openapi: 3.0.0\ninfo:\n  title: Protected\n")
	handler.SetOpenAPISpec(specContent)

	protect := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("X-Test-Auth") != "ok" {
				http.Error(w, "forbidden", http.StatusForbidden)

				return
			}

			next.ServeHTTP(w, r)
		})
	}

	mux := http.NewServeMux()
	handler.RegisterDocs(mux, protect)

	// Without auth header — blocked
	req := httptest.NewRequest(http.MethodGet, "/docs", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("unauthed /docs status = %d; want %d", rec.Code, http.StatusForbidden)
	}

	// With auth header — allowed
	req = httptest.NewRequest(http.MethodGet, "/docs", nil)
	req.Header.Set("X-Test-Auth", "ok")
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("authed /docs status = %d; want %d", rec.Code, http.StatusOK)
	}

	// Spec endpoint is always public (Scalar UI fetches it via JS without credentials)
	req = httptest.NewRequest(http.MethodGet, "/docs/openapi.yaml", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("unauthed /docs/openapi.yaml status = %d; want %d (spec should be public)", rec.Code, http.StatusOK)
	}
}

func TestDocsUI(t *testing.T) {
	mux := http.NewServeMux()
	handler.RegisterDocs(mux, nil)

	req := httptest.NewRequest(http.MethodGet, "/docs", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusOK)
	}

	ct := rec.Header().Get("Content-Type")
	if ct != "text/html; charset=utf-8" {
		t.Errorf("Content-Type = %q; want %q", ct, "text/html; charset=utf-8")
	}

	body := rec.Body.String()
	if !strings.Contains(body, "<!DOCTYPE html>") {
		t.Error("body should contain HTML doctype")
	}

	if !strings.Contains(body, "SecretDrop API") {
		t.Error("body should contain page title")
	}

	if !strings.Contains(body, "openapi.yaml") {
		t.Error("body should reference OpenAPI spec")
	}
}
