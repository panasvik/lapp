package db

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/evanoberholster/imagemeta"
	_ "github.com/mattn/go-sqlite3"
	"github.com/rwcarlsen/goexif/exif"
)

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

	walkErr = filepath.WalkDir("./assets/uploads", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		userID := 0
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		meta, err := imagemeta.Decode(file)
		var tm time.Time
		if err == nil {
			tm = meta.DigitizedDate()
		} else {
			tm = time.Now()
		}

		dateStr := tm.Format("20060102")
		dateInt, err := strconv.Atoi(dateStr)
		if err != nil {
			return err
		}

		base := filepath.Base(path)

		data <- RowData{UserID: userID, Name: base, Date: dateInt}

		return nil
	})
}

// extractEXIFDate reads a file from embed.FS and attempts to extract the date from EXIF metadata.
func extractEXIFDate(fsys embed.FS, imgPath string) (int, error) {
	file, err := fsys.Open(imgPath)
	if err != nil {
		return 0, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	x, err := exif.Decode(file)
	if err != nil {
		return 0, fmt.Errorf("EXIF metadata not found or unreadable: %w", err)
	}

	tm, err := x.DateTime()
	if err != nil {
		return 0, fmt.Errorf("date tag not found in EXIF metadata: %w", err)
	}

	dateStr := tm.Format("20060102")
	dateInt, err := strconv.Atoi(dateStr)
	if err != nil {
		return 0, fmt.Errorf("failed to convert date: %w", err)
	}

	return dateInt, nil
}
