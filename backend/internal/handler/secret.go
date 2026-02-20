package handler

import (
	"net/http"

	"github.com/bilusteknoloji/secretdrop/internal/appinfo"
	"github.com/bilusteknoloji/secretdrop/internal/model"
	"github.com/bilusteknoloji/secretdrop/internal/service"
)

// SecretHandler handles HTTP requests for secret operations.
type SecretHandler struct {
	svc *service.SecretService
}

// NewSecretHandler creates a new SecretHandler.
func NewSecretHandler(svc *service.SecretService) *SecretHandler {
	return &SecretHandler{svc: svc}
}

// Register registers the secret routes on the given mux.
func (h *SecretHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/secrets", h.Create)
	mux.HandleFunc("POST /api/v1/secrets/{token}/reveal", h.Reveal)
	mux.HandleFunc("GET /healthz", handleHealthz)
}

// Create handles POST /api/v1/secrets.
func (h *SecretHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req model.CreateRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, "validation_error", "Invalid JSON body", http.StatusBadRequest)

		return
	}

	resp, err := h.svc.Create(r.Context(), 0, &req)
	if err != nil {
		handleServiceError(w, err)

		return
	}

	writeJSON(w, http.StatusCreated, resp)
}

// Reveal handles POST /api/v1/secrets/{token}/reveal.
func (h *SecretHandler) Reveal(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	if token == "" {
		writeError(w, "validation_error", "Token is required", http.StatusBadRequest)

		return
	}

	var req model.RevealRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, "validation_error", "Invalid JSON body", http.StatusBadRequest)

		return
	}

	resp, err := h.svc.Reveal(r.Context(), token, &req)
	if err != nil {
		handleServiceError(w, err)

		return
	}

	writeJSON(w, http.StatusOK, resp)
}

func handleHealthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"version": appinfo.Version,
	})
}

func handleServiceError(w http.ResponseWriter, err error) {
	if appErr, ok := err.(*model.AppError); ok {
		writeError(w, appErr.Type, appErr.Message, appErr.StatusCode)

		return
	}

	writeError(w, "internal_error", "Internal server error", http.StatusInternalServerError)
}
