package auth

import (
	"database/sql"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	AccessTokenLiveTime  = 15 * time.Minute
	RefreshTokenLiveTime = 24 * 28 * time.Hour
)

var (
	ErrWrongTokenType = errors.New("wrong token type")
	ErrInvalidToken   = errors.New("invalid token")
)

type TokenInfo struct {
	UserID int
	Role   string
	Exp    int64
	Iat    int64
}

type Authenticator struct {
	key string
	db  sql.DB
}

func (a *Authenticator) GenerateToken(id int, role string, lifetime time.Duration) (string, error) {
	claims := jwt.MapClaims{
		"userID": id,
		"role":   role,
		"exp":    time.Now().Add(lifetime).Unix(),
		"iat":    time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	return token.SignedString(a.key)
}

func (a *Authenticator) GetUserTokens(userID int) (string, string, error) {
	accessToken, err := a.GenerateToken(userID, "user", AccessTokenLiveTime)
	if err != nil {
		return "", "", err
	}
	refreshToken, err := a.GenerateToken(userID, "user", RefreshTokenLiveTime)
	if err != nil {
		return "", "", err
	}
	return accessToken, refreshToken, err
}

func (a *Authenticator) UpdateEnv(key string, val string) {
	if key == "SECRET_KEY" {
		a.key = val
	}
}

func (a *Authenticator) ValidateToken(tokenString string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrWrongTokenType
		}
		return a.key, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, ErrInvalidToken
}

func (a *Authenticator) CheckToken(token string) (TokenInfo, error) {
	claims, err := a.ValidateToken(token)
	if err != nil {
		return TokenInfo{}, err
	}
	return TokenInfo{int(claims["userID"].(float64)), claims["role"].(string), int64(claims["exp"].(float64)), int64(claims["iat"].(float64))}, nil
}

func NewAuthenticator