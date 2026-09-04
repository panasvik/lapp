package assets

import (
	"embed"
	"fmt"
	"io/fs"
	"log"
	"path/filepath"
	"strconv"

	_ "github.com/mattn/go-sqlite3"
	"github.com/rwcarlsen/goexif/exif"
)

//go:embed uploads/*
var assetsFS embed.FS

type RowData struct {
	UserID int
	Name   string
	Date   int
}

func PopulateDB(data chan<- RowData, errChan chan<- error) {
	defer close(errChan)

	var walkErr error
	defer func() { errChan <- walkErr }()

	defer close(data)

	walkErr = fs.WalkDir(assetsFS, "uploads", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		userID := 0
		dateInt, extractErr := extractEXIFDate(assetsFS, path)
		if extractErr != nil {
			log.Printf("Предупреждение: не удалось получить EXIF дату для %s: %v. В базу будет записан 0.", path, extractErr)
			dateInt = 0
		}
		base := filepath.Base(path)

		data <- RowData{UserID: userID, Name: base, Date: dateInt}

		return nil
	})
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
