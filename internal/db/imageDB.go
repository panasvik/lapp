package db

import (
	"ImageCacheProject/internal/brocker"
	"ImageCacheProject/internal/util"
	"database/sql"
	"errors"
	"fmt"
	"log"

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
	GetNamesUser(targetDate int, userID int) (names []string, err error)
	GetDatesUser(userID int) (dates []int, err error)
	GetLibsNamesUser(userID int) (names []string, err error)
	NameBelongsToUser(path string, userID int) (bool, error)
	ProcessEvent(e brocker.Event) util.Issue
	PushLimit()
	PullLimit()
}

type ImgData struct {
	HolderID int
	Holder   ImageHolderType
	Path     string
	Date     int
	Callback func() error
}

func (db *AppDB) GetNamesUser(targetDate int, groupID int) (names []string, err error) {
	return db.getNames(targetDate, groupID, User)
}

func (db *AppDB) GetDatesUser(groupID int) (dates []int, err error) {
	return db.getDates(groupID, User)
}

func (db *AppDB) GetLibsNamesUser(groupID int) (names []string, err error) {
	return db.getLibsNames(groupID, User)
}
func (db *AppDB) NameBelongsToUser(path string, userID int) (bool, error) {
	return db.NameBelongsToHolder(path, User, userID)
}

func initAndPopulateImageDB(dbPath string) error {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return fmt.Errorf("не удалось открыть image бд: %w", err)
	}
	defer db.Close()

	createTableSQL := `
    CREATE TABLE IF NOT EXISTS images (
       userID INTEGER,
       img_path TEXT,
       date INTEGER
    );`

	if _, err := db.Exec(createTableSQL); err != nil {
		return fmt.Errorf("не удалось создать таблицу: %w", err)
	}

	createIndexSQL := `
    CREATE UNIQUE INDEX IF NOT EXISTS idx_images_user_path 
    ON images (userID, img_path);`

	if _, err := db.Exec(createIndexSQL); err != nil {
		return fmt.Errorf("не удалось создать уникальный индекс: %w", err)
	}

	query := `
    INSERT INTO images (userID, img_path, date) 
    VALUES (?, ?, ?)
    ON CONFLICT (userID, img_path) DO NOTHING`

	stmt, err := db.Prepare(query)
	if err != nil {
		return fmt.Errorf("ошибка подготовки запроса: %w", err)
	}
	defer stmt.Close()

	data := make(chan RowData, 5)
	errChan := make(chan error)
	go PopulateDB(data, errChan)
	for item := range data {
		_, err = stmt.Exec(item.HolderID, item.Name, item.Date)
		if err != nil {
			log.Printf("Предупреждение: не удалось добавить файл %s в БД: %v", item.Name, err)
		}
	}
	return <-errChan
}
