package db

import (
	"database/sql"
	"errors"
	"fmt"
	"sync/atomic"
)

var (
	ErrIMGNotFound = errors.New("DB: img not found")
	ErrNoNames     = errors.New("No images found for user by date")
)

type LoveAppDB struct {
	*sql.DB
	IsOpen atomic.Bool
}

func Connect() (*LoveAppDB, error) {
	db, err := sql.Open("sqlite", "app_data.db")
	if err != nil {
		return nil, err
	}
	d := &LoveAppDB{
		DB: db}
	d.IsOpen.Store(true)
	return d, nil
}

func (db *LoveAppDB) fastGetNames(targetDate int, userID int) (names []string, err error) {
	query := `
		SELECT img_path
		FROM images
		WHERE userID = ? AND date = ?`
	rows, err := db.Query(query, userID, userID, targetDate)
	defer rows.Close()

	if err != nil {
		return nil, fmt.Errorf("ошибка выполнения запроса: %w", err)
	}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("ошибка сканирования строки: %w", err)
		}
		names = append(names, name)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ошибка при чтении результатов: %w", err)
	}
	if len(names) == 0 {
		return nil, ErrNoNames
	}
	return names, nil
}

func (db *LoveAppDB) GetNames(targetDate int, userID int) (names []string, err error) {
	if !db.IsOpen.Load() {
		return nil, fmt.Errorf("DataBase is closed")
	}

	names, err = db.fastGetNames(targetDate, userID)
	if err == nil {
		return names, nil
	}

	// 2. SQL-запрос с подзапросом для поиска ближайшей даты
	query := `
		SELECT img_path 
		FROM images 
		WHERE userID = ? AND date = (
			SELECT date 
			FROM images 
			WHERE userID = ? 
			ORDER BY ABS(date - ?) ASC 
			LIMIT 1
		)
	`

	// 3. Выполняем запрос.
	// Обратите внимание на порядок аргументов: userID, userID, targetDate
	rows, err := db.Query(query, userID, userID, targetDate)
	if err != nil {
		return nil, fmt.Errorf("error completing request: %w", err)
	}
	defer rows.Close()

	// 4. Итерируемся по результатам
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("error parsing string: %w", err)
		}
		names = append(names, name)
	}

	// 5. Проверяем ошибки, которые могли возникнуть во время итерации
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error reading results: %w", err)
	}

	return names, nil
}
