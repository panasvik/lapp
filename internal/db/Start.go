package db

import (
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/evanoberholster/imagemeta"
	_ "github.com/mattn/go-sqlite3"
)

type RowData struct {
	HolderID int
	Name     string
	Date     int
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

		data <- RowData{HolderID: userID, Name: base, Date: dateInt}

		return nil
	})
}
