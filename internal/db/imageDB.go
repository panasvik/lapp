package db

import (
	"ImageCacheProject/internal/brocker"
	"ImageCacheProject/internal/util"
	"errors"

	_ "github.com/mattn/go-sqlite3"
)

var (
	ErrNoNames = errors.New("no images found for user by date")
	ErrUpload  = errors.New("unable to upload image")
)

const (
	maxDBReq = 10
	IDargc   = 3
)

type ImageDB interface {
	GetNames(targetDate int, userID int) (names []string, err error)
	GetDates(userID int) (dates []int, err error)
	GetLibsNames(userID int) (names []string, err error)
	GetUserIDsByImgName(path string) ([]int, error)
	ProcessEvent(e brocker.Event) util.Issue
	PushLimit()
	PullLimit()
}

type ImgData struct {
	UserID   int
	Path     string
	Date     int
	Callback func() error
}
