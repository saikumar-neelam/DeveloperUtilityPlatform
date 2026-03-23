package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/devutility/webhookplatform/internal/domain"
	"github.com/devutility/webhookplatform/internal/service"
)

type ctxUserKey struct{}

// TokenVerifier can validate a raw token string.
type TokenVerifier interface {
	Verify(token string) (*service.TokenClaims, error)
}

// Require is an HTTP middleware that enforces a valid Bearer token.
// On success it injects *domain.User into the request context.
func Require(auth TokenVerifier, users domain.UserRepo) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := bearerToken(r)
			if token == "" {
				http.Error(w, `{"error":"authentication required"}`, http.StatusUnauthorized)
				return
			}
			claims, err := auth.Verify(token)
			if err != nil {
				http.Error(w, `{"error":"invalid or expired token"}`, http.StatusUnauthorized)
				return
			}
			user, err := users.GetUserByID(r.Context(), claims.UserID)
			if err != nil {
				http.Error(w, `{"error":"user not found"}`, http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), ctxUserKey{}, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// UserFromContext extracts the authenticated user from a request context.
func UserFromContext(ctx context.Context) (*domain.User, bool) {
	u, ok := ctx.Value(ctxUserKey{}).(*domain.User)
	return u, ok && u != nil
}

func bearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if strings.HasPrefix(h, "Bearer ") {
		return strings.TrimPrefix(h, "Bearer ")
	}
	return ""
}
