package assets

import (
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"log"
	"strconv"

	_ "github.com/mattn/go-sqlite3"
	"github.com/rwcarlsen/goexif/exif"
)

//go:embed uploads/*
var assetsFS embed.FS

func start() {
	err := initAndPopulateDB("loveApp.db")
	if err != nil {
		log.Fatalf("Ошибка при работе с БД: %v", err)
	}
	fmt.Println("База данных успешно создана и заполнена!")
}

// initAndPopulateDB создает SQLite БД, таблицу и заполняет ее файлами
func initAndPopulateDB(dbPath string) error {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return fmt.Errorf("не удалось открыть бд: %w", err)
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

	entries, err := fs.ReadDir(assetsFS, "uploads")
	if err != nil {
		return fmt.Errorf("не удалось прочитать директорию uploads: %w", err)
	}

	insertSQL := `INSERT INTO images (userID, img_path, date) VALUES (?, ?, ?)`
	stmt, err := db.Prepare(insertSQL)
	if err != nil {
		return fmt.Errorf("ошибка подготовки запроса: %w", err)
	}
	defer stmt.Close()

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		imgPath := "uploads/" + entry.Name()
		// Жестко задаем userID равным 0 для всех файлов
		userID := 0

		// Извлекаем дату из EXIF
		dateInt, err := extractEXIFDate(assetsFS, imgPath)
		if err != nil {
			log.Printf("Предупреждение: не удалось получить EXIF дату для %s: %v. В базу будет записан 0.", imgPath, err)
			dateInt = 0
		}

		_, err = stmt.Exec(userID, imgPath, dateInt)
		if err != nil {
			log.Printf("Предупреждение: не удалось добавить файл %s в БД: %v", imgPath, err)
		}
	}

	return nil
}

// extractEXIFDate читает файл из embed.FS и пытается извлечь дату из EXIF
func extractEXIFDate(fsys embed.FS, imgPath string) (int, error) {
	file, err := fsys.Open(imgPath)
	if err != nil {
		return 0, fmt.Errorf("ошибка открытия файла: %w", err)
	}
	defer file.Close()

	x, err := exif.Decode(file)
	if err != nil {
		return 0, fmt.Errorf("EXIF не найден или не читается: %w", err)
	}

	tm, err := x.DateTime()
	if err != nil {
		return 0, fmt.Errorf("тег даты не найден в EXIF: %w", err)
	}

	dateStr := tm.Format("20060102")
	dateInt, err := strconv.Atoi(dateStr)
	if err != nil {
		return 0, fmt.Errorf("ошибка конвертации даты: %w", err)
	}

	return dateInt, nil
}
