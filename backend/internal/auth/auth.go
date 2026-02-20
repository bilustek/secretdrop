package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	defaultAccessExpiry  = 15 * time.Minute
	defaultRefreshExpiry = 30 * 24 * time.Hour // 30 days
)

// Option configures the auth Service.
type Option func(*Service) error

// Claims holds the JWT payload.
type Claims struct {
	UserID int64  `json:"sub"`
	Email  string `json:"email"`
	Tier   string `json:"tier"`
	jwt.RegisteredClaims
}

// TokenPair holds access and refresh tokens.
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// Service handles JWT token operations.
type Service struct {
	secret        []byte
	accessExpiry  time.Duration
	refreshExpiry time.Duration
}

// New creates a new auth Service.
func New(secret string, opts ...Option) (*Service, error) {
	if secret == "" {
		return nil, errors.New("jwt secret cannot be empty")
	}

	s := &Service{
		secret:        []byte(secret),
		accessExpiry:  defaultAccessExpiry,
		refreshExpiry: defaultRefreshExpiry,
	}

	for _, opt := range opts {
		if err := opt(s); err != nil {
			return nil, fmt.Errorf("apply option: %w", err)
		}
	}

	return s, nil
}

// WithAccessExpiry sets the access token expiry duration.
func WithAccessExpiry(d time.Duration) Option {
	return func(s *Service) error {
		if d <= 0 {
			return errors.New("access expiry must be positive")
		}

		s.accessExpiry = d

		return nil
	}
}

// WithRefreshExpiry sets the refresh token expiry duration.
func WithRefreshExpiry(d time.Duration) Option {
	return func(s *Service) error {
		if d <= 0 {
			return errors.New("refresh expiry must be positive")
		}

		s.refreshExpiry = d

		return nil
	}
}

// GenerateTokenPair creates an access/refresh token pair for the given user.
func (s *Service) GenerateTokenPair(userID int64, email, tier string) (*TokenPair, error) {
	now := time.Now()

	accessClaims := &Claims{
		UserID: userID,
		Email:  email,
		Tier:   tier,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(s.accessExpiry)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}

	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)

	accessStr, err := accessToken.SignedString(s.secret)
	if err != nil {
		return nil, fmt.Errorf("sign access token: %w", err)
	}

	refreshClaims := &Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(s.refreshExpiry)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}

	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)

	refreshStr, err := refreshToken.SignedString(s.secret)
	if err != nil {
		return nil, fmt.Errorf("sign refresh token: %w", err)
	}

	return &TokenPair{
		AccessToken:  accessStr,
		RefreshToken: refreshStr,
	}, nil
}

// VerifyToken parses and validates a JWT token string, returning the claims.
func (s *Service) VerifyToken(tokenStr string) (*Claims, error) {
	claims := &Claims{}

	token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}

		return s.secret, nil
	})
	if err != nil {
		return nil, fmt.Errorf("parse token: %w", err)
	}

	if !token.Valid {
		return nil, errors.New("invalid token")
	}

	return claims, nil
}
