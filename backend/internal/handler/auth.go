package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/devutility/webhookplatform/internal/domain"
	authmw "github.com/devutility/webhookplatform/internal/middleware"
	"github.com/devutility/webhookplatform/internal/service"
)

// AuthService is the interface the auth handler depends on.
type AuthService interface {
	Register(ctx context.Context, email, name, password string) (string, *domain.User, error)
	Login(ctx context.Context, email, password string) (string, *domain.User, error)
}

// AuthHandler holds the HTTP handlers for auth endpoints.
type AuthHandler struct {
	auth AuthService
}

func NewAuth(auth *service.AuthService) *AuthHandler {
	return &AuthHandler{auth: auth}
}

// Register handles POST /auth/register.
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Name     string `json:"name"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	token, user, err := h.auth.Register(r.Context(), body.Email, body.Name, body.Password)
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "duplicate") || strings.Contains(msg, "unique") {
			writeError(w, http.StatusConflict, "email already registered")
			return
		}
		writeError(w, http.StatusBadRequest, msg)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"token": token,
		"user":  user,
	})
}

// Login handles POST /auth/login.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	token, user, err := h.auth.Login(r.Context(), body.Email, body.Password)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"token": token,
		"user":  user,
	})
}

// Me handles GET /auth/me — returns the current user from context.
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	user, ok := authmw.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}
	writeJSON(w, http.StatusOK, user)
}
