package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/mail"
	"time"

	"github.com/bilusteknoloji/secretdrop/internal/appinfo"
	"github.com/bilusteknoloji/secretdrop/internal/email"
	"github.com/bilusteknoloji/secretdrop/internal/model"
	"github.com/bilusteknoloji/secretdrop/internal/repository"
	"github.com/bilusteknoloji/secretdrop/internal/user"

	"github.com/bilustek/secretdropvault"
	"github.com/google/uuid"
)

const (
	defaultExpiry      = 10 * time.Minute
	defaultExpiryLabel = "10m"
)

// AllowedExpiries maps user-facing expiry values to durations.
var AllowedExpiries = map[string]time.Duration{
	"10m": 10 * time.Minute,
	"1h":  1 * time.Hour,
	"1d":  24 * time.Hour,
	"5d":  5 * 24 * time.Hour,
	"10d": 10 * 24 * time.Hour,
	"30d": 30 * 24 * time.Hour,
}

// SecretService implements the core business logic for creating and revealing secrets.
type SecretService struct {
	repo          repository.Repository
	sender        email.Sender
	userRepo      user.Repository
	baseURL       string
	fromEmail     string
	expiry        time.Duration
	defaultExpiry string
}

// Option configures a SecretService value.
type Option func(*SecretService) error

// WithBaseURL sets the base URL for generated links.
func WithBaseURL(url string) Option {
	return func(s *SecretService) error {
		if url == "" {
			return errors.New("base URL cannot be empty")
		}

		s.baseURL = url

		return nil
	}
}

// WithFromEmail sets the sender email address.
func WithFromEmail(from string) Option {
	return func(s *SecretService) error {
		if from == "" {
			return errors.New("from email cannot be empty")
		}

		s.fromEmail = from

		return nil
	}
}

// WithExpiry sets the secret expiration duration.
func WithExpiry(d time.Duration) Option {
	return func(s *SecretService) error {
		if d <= 0 {
			return errors.New("expiry must be positive")
		}

		s.expiry = d

		return nil
	}
}

// WithDefaultExpiry sets the default expiry label returned to clients (e.g. "10m").
// If raw is not a key in AllowedExpiries, the closest allowed value is used.
func WithDefaultExpiry(raw string) Option {
	return func(s *SecretService) error {
		s.defaultExpiry = NormalizeExpiry(raw)

		return nil
	}
}

// NormalizeExpiry returns raw unchanged when it is a key in AllowedExpiries.
// Otherwise it parses raw as a Go duration and picks the closest allowed key.
// Falls back to "10m" when raw is empty or unparseable.
func NormalizeExpiry(raw string) string {
	if raw == "" {
		return defaultExpiryLabel
	}

	if _, ok := AllowedExpiries[raw]; ok {
		return raw
	}

	d, err := time.ParseDuration(raw)
	if err != nil {
		return defaultExpiryLabel
	}

	bestKey := "10m"
	bestDiff := abs(d - AllowedExpiries[bestKey])

	for key, dur := range AllowedExpiries {
		diff := abs(d - dur)
		if diff < bestDiff || (diff == bestDiff && dur < AllowedExpiries[bestKey]) {
			bestKey = key
			bestDiff = diff
		}
	}

	return bestKey
}

func abs(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}

	return d
}

// WithUserRepo sets the user repository for usage limit enforcement.
func WithUserRepo(r user.Repository) Option {
	return func(s *SecretService) error {
		s.userRepo = r

		return nil
	}
}

// DefaultExpiry returns the default expiry label for clients (e.g. "10m").
func (s *SecretService) DefaultExpiry() string {
	if s.defaultExpiry != "" {
		return s.defaultExpiry
	}

	return defaultExpiryLabel
}

// New creates a new SecretService with the given repository and email sender.
func New(repo repository.Repository, sender email.Sender, opts ...Option) (*SecretService, error) {
	s := &SecretService{
		repo:   repo,
		sender: sender,
		expiry: defaultExpiry,
	}

	for _, opt := range opts {
		if err := opt(s); err != nil {
			return nil, fmt.Errorf("apply option: %w", err)
		}
	}

	return s, nil
}

// Create encrypts the secret text for each recipient, stores it,
// and sends notification emails.
func (s *SecretService) Create(
	ctx context.Context,
	userID int64,
	req *model.CreateRequest,
) (*model.CreateResponse, error) {
	maxRecipients := model.ProMaxRecipients
	maxTextLength := model.ProMaxTextLength

	if s.userRepo != nil {
		u, err := s.userRepo.FindByID(ctx, userID)
		if err != nil {
			return nil, &model.AppError{
				Type:       "internal_error",
				Message:    "Failed to verify user",
				StatusCode: http.StatusInternalServerError,
			}
		}

		if tl, tlErr := s.userRepo.GetLimits(ctx, u.Tier); tlErr == nil {
			u.TierSecretsLimit = tl.SecretsLimit
			u.TierRecipientsLimit = tl.RecipientsLimit
		}

		if !u.CanCreateSecret() {
			return nil, &model.AppError{
				Type:       "limit_reached",
				Message:    "Secret creation limit reached",
				StatusCode: http.StatusForbidden,
			}
		}

		maxRecipients = u.RecipientsLimit()
		maxTextLength = u.MaxTextLength()
	}

	if err := validateCreateRequest(req, maxRecipients, maxTextLength); err != nil {
		return nil, err
	}

	expiry := s.expiry
	if req.ExpiresIn != "" {
		expiry = AllowedExpiries[req.ExpiresIn]
	}

	batchID := uuid.New().String()
	expiresAt := time.Now().Add(expiry).UTC()
	recipients := make([]model.RecipientLink, 0, len(req.To))

	var senderName string
	if s.userRepo != nil {
		if u, err := s.userRepo.FindByID(ctx, userID); err == nil {
			senderName = u.Name
		}
	}

	for _, recipientEmail := range req.To {
		link, err := s.createForRecipient(ctx, req.Text, recipientEmail, expiresAt, senderName)
		if err != nil {
			return nil, fmt.Errorf("create secret for %s: %w", recipientEmail, err)
		}

		recipients = append(recipients, *link)
	}

	if s.userRepo != nil {
		if err := s.userRepo.IncrementSecretsUsed(ctx, userID); err != nil {
			slog.Error("increment secrets used", "error", err, "user_id", userID)
			// Don't fail the request — the secret was already created
		}
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
	recipientHash := secretdropvault.HashEmail(req.Email)

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

	randomKey, err := secretdropvault.DecodeKey(req.Key)
	if err != nil {
		return nil, &model.AppError{
			Type: "decrypt_failed", Message: "Invalid key",
			StatusCode: model.StatusForbidden,
		}
	}

	finalKey, err := secretdropvault.DeriveKey(randomKey, req.Email)
	if err != nil {
		return nil, &model.AppError{
			Type: "decrypt_failed", Message: "Key derivation failed",
			StatusCode: model.StatusForbidden,
		}
	}

	plaintext, err := secretdropvault.Decrypt(finalKey, secret.EncryptedBlob, secret.Nonce)
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
	senderName string,
) (*model.RecipientLink, error) {
	randomKey, err := secretdropvault.GenerateRandomKey()
	if err != nil {
		return nil, fmt.Errorf("generate random key: %w", err)
	}

	finalKey, err := secretdropvault.DeriveKey(randomKey, recipientEmail)
	if err != nil {
		return nil, fmt.Errorf("derive key: %w", err)
	}

	ciphertext, nonce, err := secretdropvault.Encrypt(finalKey, []byte(text))
	if err != nil {
		return nil, fmt.Errorf("encrypt: %w", err)
	}

	token, err := secretdropvault.GenerateToken()
	if err != nil {
		return nil, fmt.Errorf("generate token: %w", err)
	}

	secret := &model.Secret{
		Token:         token,
		EncryptedBlob: ciphertext,
		Nonce:         nonce,
		RecipientHash: secretdropvault.HashEmail(recipientEmail),
		ExpiresAt:     expiresAt,
	}

	if err := s.repo.Store(ctx, secret); err != nil {
		return nil, fmt.Errorf("store secret: %w", err)
	}

	encodedKey := secretdropvault.EncodeKey(randomKey)
	link := fmt.Sprintf("%s/s/%s#%s", s.baseURL, token, encodedKey)

	fromLine := "A SecretDrop user"
	if senderName != "" {
		fromLine = senderName
	}

	subject := fromLine + " sent you a secure message — SecretDrop"
	html := buildNotificationEmail(fromLine, link, expiresAt)

	if err := s.sender.Send(ctx, recipientEmail, subject, html); err != nil {
		return nil, fmt.Errorf("send email: %w", err)
	}

	return &model.RecipientLink{
		Email: recipientEmail,
		Link:  link,
	}, nil
}

func validateCreateRequest(req *model.CreateRequest, maxRecipients, maxTextLength int) error {
	if req.ExpiresIn != "" {
		if _, ok := AllowedExpiries[req.ExpiresIn]; !ok {
			return &model.AppError{
				Type:       "validation_error",
				Message:    "Invalid expiry value",
				StatusCode: model.StatusUnprocessableEntity,
			}
		}
	}

	if len(req.Text) == 0 {
		return &model.AppError{
			Type:       "validation_error",
			Message:    "Text is required",
			StatusCode: model.StatusUnprocessableEntity,
		}
	}

	if len(req.Text) > maxTextLength {
		return &model.AppError{
			Type:       "text_too_long",
			Message:    fmt.Sprintf("Text exceeds maximum length of %dKB", maxTextLength/1024),
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

	if len(req.To) > maxRecipients {
		return &model.AppError{
			Type:       "too_many_recipients",
			Message:    fmt.Sprintf("Maximum %d recipients allowed", maxRecipients),
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

func buildNotificationEmail(senderName, link string, expiresAt time.Time) string {
	expiry := expiresAt.Format("Jan 2, 2006 at 3:04 PM UTC")

	return "<!DOCTYPE html>" +
		`<html lang="en">` +
		"<head><meta charset=\"UTF-8\"></head>" +
		`<body style="margin:0;padding:0;background-color:#f9fafb;` +
		`font-family:-apple-system,BlinkMacSystemFont,` +
		`'Segoe UI',Roboto,sans-serif;">` +
		`<table width="100%" cellpadding="0" cellspacing="0" ` +
		`style="background-color:#f9fafb;padding:40px 0;">` +
		"<tr><td align=\"center\">" +
		`<table width="560" cellpadding="0" cellspacing="0" ` +
		`style="background-color:#ffffff;border-radius:8px;` +
		`border:1px solid #e5e7eb;padding:40px;">` +
		"<tr><td>" +
		`<h1 style="font-size:20px;font-weight:600;` +
		`color:#111827;margin:0 0 16px;">` +
		"You have a secure message</h1>" +
		`<p style="font-size:15px;color:#374151;` +
		`line-height:1.6;margin:0 0 8px;">` +
		"<strong>" + senderName + "</strong> " +
		"shared a secure message with you via SecretDrop.</p>" +
		`<p style="font-size:14px;color:#6b7280;` +
		`line-height:1.6;margin:0 0 24px;">` +
		"This message can only be viewed once and expires on " +
		expiry + ".</p>" +
		`<table cellpadding="0" cellspacing="0" ` +
		`style="margin:0 0 24px;">` +
		`<tr><td style="background-color:#4f46e5;` +
		`border-radius:6px;padding:12px 24px;">` +
		`<a href="` + link + `" style="color:#ffffff;` +
		`text-decoration:none;font-size:15px;font-weight:500;">` +
		"View Secure Message</a>" +
		"</td></tr></table>" +
		`<p style="font-size:13px;color:#9ca3af;` +
		`line-height:1.5;margin:0 0 4px;">` +
		"You will need to verify your email address " +
		"before the message is revealed.</p>" +
		`<hr style="border:none;border-top:1px solid #e5e7eb;` +
		`margin:24px 0;">` +
		`<p style="font-size:12px;color:#9ca3af;` +
		`line-height:1.5;margin:0;">` +
		"SecretDrop encrypts messages with AES-256-GCM. " +
		"The decryption key exists only in the link above " +
		"and is never stored on our servers. " +
		`<a href="https://secretdrop.us" ` +
		`style="color:#6b7280;">secretdrop.us</a>` +
		` <span style="color:#d1d5db;">v` +
		appinfo.Version + "</span></p>" +
		"</td></tr></table>" +
		"</td></tr></table>" +
		"</body></html>"
}
