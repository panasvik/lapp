package db

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"sync/atomic"
)

const (
	maxDBReq = 50
)

var (
	ErrConversion = errors.New("unable to convert data type")
	ErrWrongTopic = errors.New("send data from wrong topic")
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

func (db *AppDB) PushLimit() {
	db.limit <- struct{}{}
}

func (db *AppDB) PullLimit() {
	<-db.limit
}
