package db

import (
	"ImageCacheProject/internal/brocker"
	"ImageCacheProject/internal/util"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/rwcarlsen/goexif/exif"
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
	callback func() error
}

func MakeImgData(path string, userID int, callback func() error) (ImgData, error) {
	file, err := os.Open(path)
	if err != nil {
		return ImgData{}, fmt.Errorf("error opening file: %w", err)
	}
	defer file.Close()

	x, err := exif.Decode(file)
	var tm time.Time
	if err == nil {
		tm, err = x.DateTime()
	}
	if err != nil {
		tm = time.Now()
	}

	dateStr := tm.Format("20060102")
	dateInt, err := strconv.Atoi(dateStr)
	if err != nil {
		return ImgData{}, fmt.Errorf("error converting date: %w", err)
	}
	imgName := filepath.Base(path)
	d := ImgData{userID, imgName, dateInt, callback}
	return d, err
}
