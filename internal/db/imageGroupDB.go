package db

import (
	"ImageCacheProject/internal/brocker"
	"ImageCacheProject/internal/util"
	"database/sql"
	"fmt"
	"log"
)

type GroupImageDB interface {
	GetNamesGroup(targetDate int, groupID int) (names []string, err error)
	GetDatesGroup(groupID int) (dates []int, err error)
	GetLibsNamesGroup(groupID int) (names []string, err error)
	NameBelongsToGroup(path string, groupID int) (bool, error)
	ProcessEvent(e brocker.Event) util.Issue
	PushLimit()
	PullLimit()
}

func (db *AppDB) GetNamesGroup(targetDate int, groupID int) (names []string, err error) {
	return db.getNames(targetDate, groupID, Group)
}
func (db *AppDB) GetDatesGroup(groupID int) (dates []int, err error) {
	return db.getDates(groupID, Group)
}
func (db *AppDB) GetLibsNamesGroup(groupID int) (names []string, err error) {
	return db.getLibsNames(groupID, Group)
}
func (db *AppDB) NameBelongsToGroup(path string, groupID int) (bool, error) {
	return db.NameBelongsToHolder(path, Group, groupID)
}

func initAndPopulateImageGroupDB(dbPath string) error {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return fmt.Errorf("не удалось открыть image бд: %w", err)
	}
	defer db.Close()

	createTableSQL := `
    CREATE TABLE IF NOT EXISTS groupImages (
       groupID INTEGER,
       img_path TEXT,
       date INTEGER
    );`

	if _, err := db.Exec(createTableSQL); err != nil {
		return fmt.Errorf("не удалось создать таблицу: %w", err)
	}

	createIndexSQL := `
    CREATE UNIQUE INDEX IF NOT EXISTS idx_images_group_path 
    ON groupImages (groupID, img_path);`

	if _, err := db.Exec(createIndexSQL); err != nil {
		return fmt.Errorf("не удалось создать уникальный индекс: %w", err)
	}

	query := `
    INSERT INTO groupImages (groupID, img_path, date) 
    VALUES (?, ?, ?)
    ON CONFLICT (groupID, img_path) DO NOTHING`

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
