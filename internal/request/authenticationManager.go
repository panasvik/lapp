package request

import (
	"ImageCacheProject/internal/brocker"
	"ImageCacheProject/internal/util"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

type authManager struct {
	authenticator *Authenticator
	tokenDB       TokenDB
	dbHandler     *brocker.Handler
}

const userIDKey contextKey = "userID"

const deviceIDKey contextKey = "deviceID"

func ContextWithUserID(ctx context.Context, userID int, hashToken string) context.Context {
	c := context.WithValue(ctx, userIDKey, userID)
	return context.WithValue(c, deviceIDKey, hashToken)
}

func UserIDFromContext(ctx context.Context) (int, bool) {
	id, ok := ctx.Value(userIDKey).(int)
	return id, ok
}

func DeviceIDFromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(deviceIDKey).(string)
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

		token, _ := extractAccessToken(r)
		data := []byte(token)
		hash := sha256.Sum256(data)
		hashString := hex.EncodeToString(hash[:])
		ctx := ContextWithUserID(r.Context(), userID, hashString)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (a *authManager) handleRefresh(w http.ResponseWriter, r *http.Request) {

	var req RefreshReq
	defer r.Body.Close()

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "wrong JSON format", http.StatusBadRequest)
		return
	}

	dbinfo, err := a.tokenDB.GetRefreshTokenInfo(req.RefreshToken)

	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	if dbinfo.IsRevoked {
		http.Error(w, "token is revoked", http.StatusUnauthorized)
		return
	}

	info, err := a.authenticator.CheckToken(req.RefreshToken)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	access, refresh, err := a.authenticator.GetUserTokens(info.UserID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	newRTokenInfo, err := a.authenticator.CheckToken(refresh)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	ud := util.TokenData{UserID: newRTokenInfo.UserID, DeviceName: dbinfo.DeviceName, RefreshToken: refresh, Exp: newRTokenInfo.Exp, Iat: newRTokenInfo.Iat}
	a.dbHandler.Publish(ud, brocker.InsertNewToken)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(RefreshResp{RefreshToken: refresh, AccessToken: access})
}
