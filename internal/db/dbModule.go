package db

import (
	"ImageCacheProject/internal/brocker"
	"ImageCacheProject/internal/util"
	"database/sql"
	"fmt"
	"log"
	"sync/atomic"
)

const (
	maxDBReq = 50
)

const dbPath = "/data/loveApp.db"

type AppDB struct {
	*sql.DB
	IsOpen atomic.Bool
	limit  chan struct{}
}

func Connect() (*AppDB, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}
	d := &AppDB{
		DB: db, limit: make(chan struct{}, maxDBReq)}
	d.IsOpen.Store(true)
	return d, nil
}

func ColdBoot() {
	err := initAndPopulateImageDB(dbPath)
	if err != nil {
		log.Fatalf("Ошибка при работе с БД: %v", err)
	}
	err = initTokenTable(dbPath)
	if err != nil {
		log.Fatalf("Ошибка при работе с БД: %v", err)
	}
	err = initUserDB(dbPath)
	if err != nil {
		log.Fatalf("Ошибка при работе с БД: %v", err)
	}
	fmt.Println("База данных успешно создана и заполнена!")
}

type ErrIssue struct {
	err  error
	desc string
}

func (e *ErrIssue) GetErr() error {
	return e.err
}

func (e *ErrIssue) GetBody() string {
	return e.desc
}

func (e *ErrIssue) GetFixCallback() func() error {
	return nil
}

type FileRemove struct {
	err      error
	fileName string
	callback func() error
}

func (f *FileRemove) GetErr() error {
	return f.err
}

func (f *FileRemove) GetBody() string {
	return f.fileName
}

func (f *FileRemove) GetFixCallback() func() error {
	return f.callback
}

func convertBoolToInt(b bool) int {
	switch b {
	case true:
		return 1
	case false:
		return 0
	default:
		return -1
	}
}

func (db *AppDB) performDBTask(data any, topic brocker.DBTopic) util.Issue {
	switch topic {
	case brocker.InsertNewImage:
		imgd := data.(ImgData)
		err := db.insertImage(imgd.Path, imgd.HolderID, imgd.Holder, imgd.Date)
		if err != nil {
			return &FileRemove{ErrUpload, imgd.Path, imgd.Callback}
		}
		return nil
	case brocker.InsertNewToken:
		userd := data.(TokenData)
		err := db.InsertRefreshToken(userd.UserID, userd.DeviceName, userd.RefreshToken, userd.Exp, userd.Iat, userd.IsRevoked)
		return &ErrIssue{err, ""}
	case brocker.RevokeToken:
		logout := data.(UserLogOut)
		err := db.LogOutUser(logout.UserID, logout.DeviceName)
		return &ErrIssue{err, ""}

	}
	return &ErrIssue{brocker.ErrUnknownDBTopic, "unable to form userData"}
}

func (db *AppDB) PushLimit() {
	db.limit <- struct{}{}
}

func (db *AppDB) PullLimit() {
	<-db.limit
}

func (db *AppDB) ProcessEvent(e brocker.Event) util.Issue {
	data := e.GetData()
	if data == nil {
		return &ErrIssue{brocker.ErrNoDataInEvent, ""}
	}
	return db.performDBTask(data, e.GetTopic())
}
