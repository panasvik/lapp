package db

import (
	"ImageCacheProject/internal/brocker"
	"ImageCacheProject/internal/util"
)

const (
	TDargc = 5
)

type TokenDB interface {
	GetRefreshTokenInfo(refreshToken string) (TokenData, error)
	ProcessEvent(e brocker.Event) util.Issue
	PushLimit()
	PullLimit()
}

type TokenData struct {
	UserID       int
	DeviceName   string
	RefreshToken string
	Exp          int64
	Iat          int64
	IsRevoked    bool
}
