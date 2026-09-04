package db

import (
	"ImageCacheProject/internal/brocker"
	"ImageCacheProject/internal/util"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"sync/atomic"
)

var (
	ErrTokenNotFound = errors.New("token not found")
)

type UserDB struct {
	*sql.DB
	IsOpen atomic.Bool
}

type UserToken struct {
	UserID    int
	TokenHash string
	Exp       int
	Iat       int
	IsRevoked bool
}

func convertBoolToInt(b bool) int {
	switch b {
	case true:
		return 1
	case false:
		return 0
	default:
		return -1
	}
}

func ConnectToUserDB() (*UserDB, error) {
	db, err := sql.Open("sqlite3", "loveApp.db")
	if err != nil {
		return nil, err
	}
	d := &UserDB{
		DB: db}
	d.IsOpen.Store(true)
	return d, nil
}

func InitDB(dbPath string) error {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return fmt.Errorf("не удалось открыть бд: %w", err)
	}
	defer db.Close()

	createTableSQL := `
	CREATE TABLE IF NOT EXISTS refresh_tokens (
		rowID INTEGER PRIMARY KEY AUTOINCREMENT, 
		userID INTEGER,
	    tokenHash TEXT, 
		exp INTEGER,
		iat INTEGER,
		isRevoked INTEGER
	);`
	if _, err := db.Exec(createTableSQL); err != nil {
		return fmt.Errorf("unable to create a table: %w", err)
	}
	return nil
}

func (u *UserDB) InsertRefreshToken(userID int, refreshToken string, exp int, iat int, isRevoked bool) error {
	data := []byte(refreshToken)
	hash := sha256.Sum256(data)

	query := `INSERT INTO refresh_tokens (userID, tokenHash, exp, iat, isRevoked) VALUES (?, ?, ?, ?, ?)`
	_, err := u.Exec(query, userID, hash, exp, iat, convertBoolToInt(isRevoked))
	return err
}

func (u *UserDB) InsertQuery(q string) util.Issue {
	userd, err := brocker.FormUserData(q)
	if err != nil {
		return &FileRemove{err, ""}
	}
	err = u.InsertRefreshToken(userd.UserID, userd.RefreshToken, userd.Exp, userd.Iat, userd.IsRevoked)
	return &FileRemove{err, ""}
}

func (u *UserDB) GetRefreshTokenInfo(refreshToken string) (UserToken, error) {
	data := []byte(refreshToken)
	hash := sha256.Sum256(data)
	hashString := hex.EncodeToString(hash[:])
	query := `SELECT userID, tokenHash, exp, iat, isRevoked FROM refresh_tokens WHERE tokenHash = ?`
	var token UserToken
	err := u.QueryRow(query, hashString).Scan(
		&token.UserID,
		&token.TokenHash,
		&token.Exp,
		&token.Iat,
		&token.IsRevoked)
	if err != nil {
		//if errors.Is(err, sql.ErrNoRows) {
		//	return UserToken{}, ErrTokenNotFound
		//}
		return UserToken{}, err
	}
	return token, nil
}
