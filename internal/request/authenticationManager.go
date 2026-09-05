package request

import (
	"context"
	"errors"
	"net/http"
	"strings"
)

type authManager struct {
	authenticator *Authenticator
}

type contextKey string

const userIDKey contextKey = "userID"

type Role string

const UserRole Role = "user"
const AdminRole Role = "admin"

type userInfo struct {
	userID int
	role   Role
}

func ContextWithUserID(ctx context.Context, userID int) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}

func UserIDFromContext(ctx context.Context) (int, bool) {
	id, ok := ctx.Value(userIDKey).(int)
	return id, ok
}

func extractAccessToken(r *http.Request) (string, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return "", ErrAuthHeaderMissing
	}

	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		return "", ErrWrongHeaderFormat
	}

	return parts[1], nil
}

func (a *authManager) checkAccessToken(r *http.Request) (int, error) {
	token, err := extractAccessToken(r)
	if err != nil {
		return -1, err
	}
	info, err := a.authenticator.CheckToken(token)
	if err != nil {
		return -1, ErrInvalidToken
	}
	return info.UserID, nil
}

func (a *authManager) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, err := a.checkAccessToken(r)
		switch {
		case errors.Is(err, ErrAuthHeaderMissing), errors.Is(err, ErrWrongHeaderFormat), errors.Is(err, ErrWrongTokenType):
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		case errors.Is(err, ErrInvalidToken):
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		case err != nil:
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		ctx := ContextWithUserID(r.Context(), userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
