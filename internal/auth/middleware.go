package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"
)

type contextKey string

const claimsContextKey contextKey = "authClaims"

func AuthMiddleware(tokenManager *TokenManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, err := AuthenticateRequest(r, tokenManager)
			if err != nil {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), claimsContextKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func AuthenticateRequest(r *http.Request, tokenManager *TokenManager) (*Claims, error) {
	token, err := ExtractBearerToken(r.Header.Get("Authorization"))
	if err != nil {
		return nil, err
	}
	return tokenManager.ValidateToken(token, TokenTypeAccess)
}

func AuthenticateWebSocket(r *http.Request, tokenManager *TokenManager) (*Claims, error) {
	if token := strings.TrimSpace(r.URL.Query().Get("token")); token != "" {
		return tokenManager.ValidateToken(token, TokenTypeAccess)
	}
	return AuthenticateRequest(r, tokenManager)
}

func ExtractBearerToken(header string) (string, error) {
	parts := strings.SplitN(strings.TrimSpace(header), " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || strings.TrimSpace(parts[1]) == "" {
		return "", errors.New("missing bearer token")
	}
	return strings.TrimSpace(parts[1]), nil
}

func ClaimsFromContext(ctx context.Context) (*Claims, bool) {
	claims, ok := ctx.Value(claimsContextKey).(*Claims)
	return claims, ok
}
