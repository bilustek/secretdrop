package handler

import (
	"fmt"
	"net/http"
	"net/mail"
	"strings"

	"github.com/bilusteknoloji/secretdrop/internal/email"
)

const contactRecipient = "support@bilustek.com"

type contactRequest struct {
	Name    string `json:"name"`
	Email   string `json:"email"`
	Message string `json:"message"`
}

// NewContactHandler returns a handler for POST /api/v1/contact.
func NewContactHandler(sender email.Sender) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req contactRequest
		if err := readJSON(r, &req); err != nil {
			writeError(w, "validation_error", "Invalid JSON body", http.StatusBadRequest)

			return
		}

		req.Name = strings.TrimSpace(req.Name)
		req.Email = strings.TrimSpace(req.Email)
		req.Message = strings.TrimSpace(req.Message)

		if req.Name == "" || req.Email == "" || req.Message == "" {
			writeError(w, "validation_error", "Name, email, and message are required", http.StatusBadRequest)

			return
		}

		if _, err := mail.ParseAddress(req.Email); err != nil {
			writeError(w, "validation_error", "Invalid email address", http.StatusBadRequest)

			return
		}

		subject := fmt.Sprintf("[SecretDrop Contact] from %s", req.Name)
		body := fmt.Sprintf(
			"<p><strong>From:</strong> %s &lt;%s&gt;</p><p><strong>Message:</strong></p><p>%s</p>",
			req.Name, req.Email, strings.ReplaceAll(req.Message, "\n", "<br>"),
		)

		if err := sender.Send(r.Context(), contactRecipient, subject, body); err != nil {
			writeError(w, "internal_error", "Failed to send message", http.StatusInternalServerError)

			return
		}

		writeJSON(w, http.StatusOK, map[string]string{"status": "sent"})
	}
}
