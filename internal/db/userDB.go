package db

import (
	"ImageCacheProject/internal/brocker"
	"ImageCacheProject/internal/util"
	"errors"
)

const (
	ULargc = 2
)

var (
	ErrUserAlreadyExists = errors.New("user already exists")
)

type UserDB interface {
	RegisterNewUser(userName string, password string) (int, error)
	CheckUserPassword(userName string, password string) (int, error)
	ProcessEvent(e brocker.Event) util.Issue
	PushLimit()
	PullLimit()
}

type UserLogOut struct {
	UserID     int
	DeviceName string
}
