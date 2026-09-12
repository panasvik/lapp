package db

import (
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
	err = initAndPopulateImageGroupDB(dbPath)
	if err != nil {
		log.Fatalf("Ошибка при работе с БД: %v", err)
	}
	err = initGroupDB(dbPath)
	if err != nil {
		log.Fatalf("Ошибка при работе с БД: %v", err)
	}
	err = initMessagesDB(dbPath)
	if err != nil {
		log.Fatalf("Ошибка при работе с БД: %v", err)
	}
	err = initWBSubDB(dbPath)
	if err != nil {
		log.Fatalf("Ошибка при работе с БД: %v", err)
	}
	fmt.Println("База данных успешно создана и заполнена!")
}

func (db *AppDB) PushLimit() {
	db.limit <- struct{}{}
}

func (db *AppDB) PullLimit() {
	<-db.limit
}
