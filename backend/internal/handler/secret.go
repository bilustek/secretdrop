package handler

import (
	"errors"
	"net/http"
	"time"

	"github.com/bilusteknoloji/secretdrop/internal/appinfo"
	"github.com/bilusteknoloji/secretdrop/internal/middleware"
	"github.com/bilusteknoloji/secretdrop/internal/model"
	"github.com/bilusteknoloji/secretdrop/internal/service"
	"github.com/bilusteknoloji/secretdrop/internal/user"
)

// SecretHandler handles HTTP requests for secret operations.
type SecretHandler struct {
	svc      *service.SecretService
	userRepo user.Repository
}

// NewSecretHandler creates a new SecretHandler.
func NewSecretHandler(svc *service.SecretService, userRepo user.Repository) *SecretHandler {
	return &SecretHandler{svc: svc, userRepo: userRepo}
}

// Register registers the secret routes on the given mux.
func (h *SecretHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/secrets", h.Create)
	mux.HandleFunc("POST /api/v1/secrets/{token}/reveal", h.Reveal)
	mux.HandleFunc("GET /api/v1/me", h.Me)
	mux.HandleFunc("PUT /api/v1/me/timezone", h.UpdateTimezone)
	mux.HandleFunc("GET /healthz", handleHealthz)
}

// Create handles POST /api/v1/secrets.
func (h *SecretHandler) Create(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.UserFromContext(r.Context())
	if !ok {
		writeError(w, "unauthorized", "Authentication required", http.StatusUnauthorized)

		return
	}

	var req model.CreateRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, errTypeValidaton, "Invalid JSON body", http.StatusBadRequest)

		return
	}

	resp, err := h.svc.Create(r.Context(), claims.UserID, &req)
	if err != nil {
		handleServiceError(w, err)

		return
	}

	writeJSON(w, http.StatusCreated, resp)
}

// Me handles GET /api/v1/me.
func (h *SecretHandler) Me(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.UserFromContext(r.Context())
	if !ok {
		writeError(w, "unauthorized", "Authentication required", http.StatusUnauthorized)

		return
	}

	u, err := h.userRepo.FindByID(r.Context(), claims.UserID)
	if err != nil {
		writeError(w, errTypeNotFound, "User not found", http.StatusNotFound)

		return
	}

	if tl, tlErr := h.userRepo.GetLimits(r.Context(), u.Tier); tlErr == nil {
		u.TierSecretsLimit = tl.SecretsLimit
		u.TierRecipientsLimit = tl.RecipientsLimit
	}

	writeJSON(w, http.StatusOK, model.MeResponse{
		Email:           u.Email,
		Name:            u.Name,
		AvatarURL:       u.AvatarURL,
		Tier:            u.Tier,
		SecretsUsed:     u.SecretsUsed,
		SecretsLimit:    u.SecretsLimit(),
		RecipientsLimit: u.RecipientsLimit(),
		MaxTextLength:   u.MaxTextLength(),
		DefaultExpiry:   h.svc.DefaultExpiry(),
		Timezone:        u.Timezone,
	})
}

// Reveal handles POST /api/v1/secrets/{token}/reveal.
func (h *SecretHandler) Reveal(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	if token == "" {
		writeError(w, errTypeValidaton, "Token is required", http.StatusBadRequest)

		return
	}

	var req model.RevealRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, errTypeValidaton, "Invalid JSON body", http.StatusBadRequest)

		return
	}

	resp, err := h.svc.Reveal(r.Context(), token, &req)
	if err != nil {
		handleServiceError(w, err)

		return
	}

	writeJSON(w, http.StatusOK, resp)
}

// UpdateTimezone handles PUT /api/v1/me/timezone.
func (h *SecretHandler) UpdateTimezone(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.UserFromContext(r.Context())
	if !ok {
		writeError(w, "unauthorized", "Authentication required", http.StatusUnauthorized)

		return
	}

	var req model.TimezoneRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, errTypeValidaton, "Invalid JSON body", http.StatusBadRequest)

		return
	}

	if _, err := time.LoadLocation(req.Timezone); err != nil {
		writeError(w, errTypeValidaton, "Invalid timezone", http.StatusBadRequest)

		return
	}

	if err := h.userRepo.UpdateTimezone(r.Context(), claims.UserID, req.Timezone); err != nil {
		writeError(w, errTypeInternal, "Failed to update timezone", http.StatusInternalServerError)

		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func handleHealthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"version": appinfo.Version,
	})
}

func handleServiceError(w http.ResponseWriter, err error) {
	var appErr *model.AppError
	if errors.As(err, &appErr) {
		writeError(w, appErr.Type, appErr.Message, appErr.StatusCode)

		return
	}

	writeError(w, errTypeInternal, "Internal server error", http.StatusInternalServerError)
}
