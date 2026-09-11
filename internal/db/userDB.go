package db

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
)

const (
	ULargc = 2
)

var (
	ErrUserAlreadyExists = errors.New("user already exists")
)

type UserDB struct {
	*AppDB
}

func initUserDB(dbPath string) error {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return fmt.Errorf("не удалось открыть user бд: %w", err)
	}
	defer db.Close()

	createTableSQL := `
	CREATE TABLE IF NOT EXISTS users (
		userID INTEGER PRIMARY KEY AUTOINCREMENT,
	    userName TEXT, 
		passwordHash TEXT
	);`
	if _, err := db.Exec(createTableSQL); err != nil {
		return fmt.Errorf("unable to create user table: %w", err)
	}
	return nil
}

func (db *UserDB) RegisterNewUser(userName string, password string) (int, error) {
	if db.userExists(userName) {
		return -1, fmt.Errorf("%w by the name of %s", ErrUserAlreadyExists, userName)
	}
	data := []byte(password)
	hash := sha256.Sum256(data)
	hashString := hex.EncodeToString(hash[:])
	query := `INSERT INTO users (userName, passwordHash) VALUES (?, ?)`
	res, err := db.Exec(query, userName, hashString)
	if err != nil {
		return -1, err
	}
	userID, err := res.LastInsertId()
	if err != nil {
		return -1, fmt.Errorf("unable to get userID %w", err)
	}
	return int(userID), nil
}

func (db *UserDB) userExists(userName string) bool {
	query := `SELECT userID FROM users WHERE userName = ?`
	var userID int
	err := db.QueryRow(query, userName).Scan(&userID)
	return err == nil
}

func (db *UserDB) GetUserIDByCredentials(userName string, password string) (int, error) {
	data := []byte(password)
	hash := sha256.Sum256(data)
	hashString := hex.EncodeToString(hash[:])
	query := `SELECT userID FROM users WHERE userName = ? AND passwordHash = ?`
	var userID int
	err := db.QueryRow(query, userName, hashString).Scan(&userID)
	return userID, err
}
