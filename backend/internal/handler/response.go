package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/bilustek/secretdrop/internal/model"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("encode json response", "error", err)
	}
}

func writeError(w http.ResponseWriter, errType, message string, status int) {
	writeJSON(w, status, model.ErrorResponse{
		Error: model.ErrorDetail{Type: errType, Message: message},
	})
}

func readJSON(r *http.Request, v any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	return dec.Decode(v)
}
