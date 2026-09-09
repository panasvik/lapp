package db

import (
	"fmt"
)

type ImageHolderType int

const (
	User  = ImageHolderType(0)
	Group = ImageHolderType(1)
)

func (db *AppDB) fastGetNames(targetDate int, holderID int, holder ImageHolderType) (names []string, err error) {
	var query string
	if holder == User {
		query = `
		SELECT img_path
		FROM images
		WHERE userID = ? AND date = ?`
	} else {
		query = `
		SELECT img_path
		FROM groups
		WHERE groupID = ? AND date = ?`
	}
	rows, err := db.Query(query, holderID, targetDate)

	if err != nil {
		return nil, fmt.Errorf("ошибка выполнения запроса: %w", err)
	}

	defer rows.Close()

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

func (db *AppDB) getDates(holderId int, holder ImageHolderType) (dates []int, err error) {
	var query string
	if holder == User {
		query = `
		SELECT date
		FROM images
		WHERE userID = ?`
	} else {
		query = `
		SELECT date
		FROM groups
		WHERE groupID = ?`
	}
	rows, err := db.Query(query, holderId)

	if err != nil {
		return nil, fmt.Errorf("ошибка выполнения запроса: %w", err)
	}

	defer rows.Close()

	for rows.Next() {
		var date int
		if err := rows.Scan(&date); err != nil {
			return nil, fmt.Errorf("ошибка сканирования строки: %w", err)
		}
		dates = append(dates, date)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ошибка при чтении результатов: %w", err)
	}
	if len(dates) == 0 {
		return nil, ErrNoNames
	}
	return dates, nil
}

func (db *AppDB) getNames(targetDate int, holderID int, holder ImageHolderType) (names []string, err error) {
	if !db.IsOpen.Load() {
		return nil, fmt.Errorf("DataBase is closed")
	}

	names, err = db.fastGetNames(targetDate, holderID, holder)
	if err == nil {
		return names, nil
	}

	var query string
	if holder == User {
		query = `
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
	} else {
		query = `
		SELECT img_path 
		FROM groups 
		WHERE groupID = ? AND date = (
			SELECT date 
			FROM groups 
			WHERE groupID = ? 
			ORDER BY ABS(date - ?) ASC 
			LIMIT 1
		)
	`

	}
	rows, err := db.Query(query, holderID, holderID, targetDate)
	if err != nil {
		return nil, fmt.Errorf("error completing request: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("error parsing string: %w", err)
		}
		names = append(names, name)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error reading results: %w", err)
	}

	return names, nil
}

func (db *AppDB) getLibsNames(holderID int, holder ImageHolderType) (names []string, err error) {
	if !db.IsOpen.Load() {
		return nil, fmt.Errorf("DataBase is closed")
	}

	var query string
	if holder == User {
		query = `
		SELECT img_path 
		FROM images 
		WHERE userID = ? 
		ORDER BY date ASC 
	`
	} else {
		query = `
		SELECT img_path 
		FROM groups 
		WHERE groupID = ? 
		ORDER BY date ASC 
	`
	}

	rows, err := db.Query(query, holderID)
	if err != nil {
		return nil, fmt.Errorf("error completing request: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("error parsing string: %w", err)
		}
		names = append(names, name)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error reading results: %w", err)
	}

	return names, nil
}

func (db *AppDB) insertImage(path string, holderID int, holder ImageHolderType, date int) error {
	var query string
	if holder == User {
		query = `
    	INSERT INTO images (userID, img_path, date) 
    	VALUES (?, ?, ?) 
    	ON CONFLICT (userID, img_path) DO NOTHING
	`
	} else {
		query = `
    	INSERT INTO groups (groupID, img_path, date) 
    	VALUES (?, ?, ?) 
    	ON CONFLICT (groupID, img_path) DO NOTHING
    	`
	}

	_, err := db.Exec(query, holderID, path, date)
	return err
}

func (db *AppDB) NameBelongsToHolder(path string, holder ImageHolderType, holderID int) (bool, error) {
	var query string
	if holder == User {
		query = `SELECT * FROM images WHERE img_path = ? AND userID = ?`
	} else {
		query = `SELECT * FROM groups WHERE img_path = ? AND groupID = ?`
	}
	rows, err := db.Query(query, path, holderID)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	hasName := false
	for rows.Next() {
		hasName = true
		break
	}

	if err := rows.Err(); err != nil {
		return false, fmt.Errorf("error reading results: %w", err)
	}
	return hasName, nil
}
