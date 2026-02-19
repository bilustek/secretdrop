package service

import (
	"context"
	"fmt"
	"net/mail"
	"time"

	"github.com/bilusteknoloji/secretdrop/internal/crypt"
	"github.com/bilusteknoloji/secretdrop/internal/email"
	"github.com/bilusteknoloji/secretdrop/internal/model"
	"github.com/bilusteknoloji/secretdrop/internal/repository"

	"github.com/google/uuid"
)

// SecretService implements the core business logic for creating and revealing secrets.
type SecretService struct {
	repo      repository.Repository
	sender    email.Sender
	baseURL   string
	fromEmail string
	expiry    time.Duration
}

// NewSecretService creates a new SecretService.
func NewSecretService(
	repo repository.Repository,
	sender email.Sender,
	baseURL string,
	fromEmail string,
	expiry time.Duration,
) *SecretService {
	return &SecretService{
		repo:      repo,
		sender:    sender,
		baseURL:   baseURL,
		fromEmail: fromEmail,
		expiry:    expiry,
	}
}

// Create encrypts the secret text for each recipient, stores it,
// and sends notification emails.
func (s *SecretService) Create(
	ctx context.Context,
	req *model.CreateRequest,
) (*model.CreateResponse, error) {
	if err := validateCreateRequest(req); err != nil {
		return nil, err
	}

	batchID := uuid.New().String()
	expiresAt := time.Now().Add(s.expiry).UTC()
	recipients := make([]model.RecipientLink, 0, len(req.To))

	for _, recipientEmail := range req.To {
		link, err := s.createForRecipient(ctx, req.Text, recipientEmail, expiresAt)
		if err != nil {
			return nil, fmt.Errorf("create secret for %s: %w", recipientEmail, err)
		}

		recipients = append(recipients, *link)
	}

	return &model.CreateResponse{
		ID:         batchID,
		ExpiresAt:  expiresAt,
		Recipients: recipients,
	}, nil
}

// Reveal decrypts and returns the secret text, then deletes it from the database.
func (s *SecretService) Reveal(
	ctx context.Context,
	token string,
	req *model.RevealRequest,
) (*model.RevealResponse, error) {
	recipientHash := crypt.HashEmail(req.Email)

	secret, err := s.repo.FindByTokenAndHash(ctx, token, recipientHash)
	if err != nil {
		return nil, &model.AppError{
			Type: "not_found", Message: "Secret not found",
			StatusCode: model.StatusNotFound,
		}
	}

	if time.Now().After(secret.ExpiresAt) {
		_ = s.repo.Delete(ctx, secret.ID)

		return nil, &model.AppError{
			Type: "expired", Message: "Secret has expired",
			StatusCode: model.StatusGone,
		}
	}

	if secret.Viewed {
		_ = s.repo.Delete(ctx, secret.ID)

		return nil, &model.AppError{
			Type: "already_viewed", Message: "Secret has already been viewed",
			StatusCode: model.StatusGone,
		}
	}

	randomKey, err := crypt.DecodeKey(req.Key)
	if err != nil {
		return nil, &model.AppError{
			Type: "decrypt_failed", Message: "Invalid key",
			StatusCode: model.StatusForbidden,
		}
	}

	finalKey, err := crypt.DeriveKey(randomKey, req.Email)
	if err != nil {
		return nil, &model.AppError{
			Type: "decrypt_failed", Message: "Key derivation failed",
			StatusCode: model.StatusForbidden,
		}
	}

	plaintext, err := crypt.Decrypt(finalKey, secret.EncryptedBlob, secret.Nonce)
	if err != nil {
		return nil, &model.AppError{
			Type: "decrypt_failed", Message: "Decryption failed",
			StatusCode: model.StatusForbidden,
		}
	}

	if err := s.repo.Delete(ctx, secret.ID); err != nil {
		return nil, fmt.Errorf("delete secret after reveal: %w", err)
	}

	return &model.RevealResponse{Text: string(plaintext)}, nil
}

func (s *SecretService) createForRecipient(
	ctx context.Context,
	text, recipientEmail string,
	expiresAt time.Time,
) (*model.RecipientLink, error) {
	randomKey, err := crypt.GenerateRandomKey()
	if err != nil {
		return nil, fmt.Errorf("generate random key: %w", err)
	}

	finalKey, err := crypt.DeriveKey(randomKey, recipientEmail)
	if err != nil {
		return nil, fmt.Errorf("derive key: %w", err)
	}

	ciphertext, nonce, err := crypt.Encrypt(finalKey, []byte(text))
	if err != nil {
		return nil, fmt.Errorf("encrypt: %w", err)
	}

	token, err := crypt.GenerateToken()
	if err != nil {
		return nil, fmt.Errorf("generate token: %w", err)
	}

	secret := &model.Secret{
		Token:         token,
		EncryptedBlob: ciphertext,
		Nonce:         nonce,
		RecipientHash: crypt.HashEmail(recipientEmail),
		ExpiresAt:     expiresAt,
	}

	if err := s.repo.Store(ctx, secret); err != nil {
		return nil, fmt.Errorf("store secret: %w", err)
	}

	encodedKey := crypt.EncodeKey(randomKey)
	link := fmt.Sprintf("%s/s/%s#%s", s.baseURL, token, encodedKey)

	subject := "You've received a secret via SecretDrop"
	html := "<p>Someone shared a secret with you.</p>" +
		`<p><a href="` + link + `">Click here to reveal it</a></p>` +
		"<p>This link will expire and can only be used once.</p>"

	if err := s.sender.Send(ctx, recipientEmail, subject, html); err != nil {
		return nil, fmt.Errorf("send email: %w", err)
	}

	return &model.RecipientLink{
		Email: recipientEmail,
		Link:  link,
	}, nil
}

func validateCreateRequest(req *model.CreateRequest) error {
	if len(req.Text) == 0 {
		return &model.AppError{
			Type:       "validation_error",
			Message:    "Text is required",
			StatusCode: model.StatusUnprocessableEntity,
		}
	}

	if len(req.Text) > model.MaxTextLength {
		return &model.AppError{
			Type:       "text_too_long",
			Message:    "Text exceeds maximum length of 4KB",
			StatusCode: model.StatusUnprocessableEntity,
		}
	}

	if len(req.To) == 0 {
		return &model.AppError{
			Type:       "validation_error",
			Message:    "At least one recipient is required",
			StatusCode: model.StatusUnprocessableEntity,
		}
	}

	if len(req.To) > model.MaxRecipients {
		return &model.AppError{
			Type:       "too_many_recipients",
			Message:    "Maximum 5 recipients allowed",
			StatusCode: model.StatusUnprocessableEntity,
		}
	}

	for _, addr := range req.To {
		if _, err := mail.ParseAddress(addr); err != nil {
			return &model.AppError{
				Type:       "invalid_email",
				Message:    fmt.Sprintf("Invalid email address: %s", addr),
				StatusCode: model.StatusUnprocessableEntity,
			}
		}
	}

	return nil
}
