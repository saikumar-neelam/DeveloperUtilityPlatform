package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/devutility/webhookplatform/internal/domain"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// AuthService handles registration, login, and token operations.
type AuthService struct {
	users  domain.UserRepo
	secret []byte
	log    *slog.Logger
}

func NewAuth(users domain.UserRepo, jwtSecret string, log *slog.Logger) *AuthService {
	return &AuthService{users: users, secret: []byte(jwtSecret), log: log}
}

// TokenClaims is the payload encoded in the token.
type TokenClaims struct {
	UserID string `json:"uid"`
	Email  string `json:"email"`
	Exp    int64  `json:"exp"` // Unix timestamp
}

// Register creates a new user and returns a signed token.
func (s *AuthService) Register(ctx context.Context, email, name, password string) (string, *domain.User, error) {
	if email == "" || name == "" || password == "" {
		return "", nil, fmt.Errorf("email, name, and password are required")
	}
	if len(password) < 8 {
		return "", nil, fmt.Errorf("password must be at least 8 characters")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", nil, fmt.Errorf("hash password: %w", err)
	}

	u := &domain.User{
		ID:           uuid.NewString(),
		Email:        strings.ToLower(strings.TrimSpace(email)),
		Name:         strings.TrimSpace(name),
		PasswordHash: string(hash),
		Plan:         "free",
		CreatedAt:    time.Now().UTC(),
	}
	if err := s.users.CreateUser(ctx, u); err != nil {
		return "", nil, fmt.Errorf("register: %w", err)
	}

	token, err := s.sign(u)
	if err != nil {
		return "", nil, err
	}
	s.log.Info("user registered", "id", u.ID, "email", u.Email)
	return token, u, nil
}

// Login verifies credentials and returns a signed token.
func (s *AuthService) Login(ctx context.Context, email, password string) (string, *domain.User, error) {
	u, err := s.users.GetUserByEmail(ctx, strings.ToLower(strings.TrimSpace(email)))
	if err != nil {
		return "", nil, fmt.Errorf("invalid credentials")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err != nil {
		return "", nil, fmt.Errorf("invalid credentials")
	}
	token, err := s.sign(u)
	if err != nil {
		return "", nil, err
	}
	return token, u, nil
}

// Verify parses and validates a token, returning the claims.
func (s *AuthService) Verify(token string) (*TokenClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return nil, fmt.Errorf("malformed token")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, fmt.Errorf("decode payload: %w", err)
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("decode sig: %w", err)
	}
	expected := s.hmac(payload)
	if !hmac.Equal(sig, expected) {
		return nil, fmt.Errorf("invalid signature")
	}
	var claims TokenClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, fmt.Errorf("parse claims: %w", err)
	}
	if time.Now().Unix() > claims.Exp {
		return nil, fmt.Errorf("token expired")
	}
	return &claims, nil
}

func (s *AuthService) sign(u *domain.User) (string, error) {
	claims := TokenClaims{
		UserID: u.ID,
		Email:  u.Email,
		Exp:    time.Now().Add(30 * 24 * time.Hour).Unix(), // 30 days
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("marshal claims: %w", err)
	}
	enc := base64.RawURLEncoding.EncodeToString(payload)
	sig := base64.RawURLEncoding.EncodeToString(s.hmac(payload))
	return enc + "." + sig, nil
}

func (s *AuthService) hmac(data []byte) []byte {
	mac := hmac.New(sha256.New, s.secret)
	mac.Write(data)
	return mac.Sum(nil)
}
