package db

import (
	"ImageCacheProject/internal/brocker"
	"ImageCacheProject/internal/util"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"
)

const (
	TDargc = 5
)

type TokenDB interface {
	GetRefreshTokenInfo(refreshToken string) (TokenData, error)
	ProcessEvent(e brocker.Event) util.Issue
	PushLimit()
	PullLimit()
}

type TokenData struct {
	UserID       int
	DeviceName   string
	RefreshToken string
	Exp          int64
	Iat          int64
	IsRevoked    bool
}

func initTokenTable(dbPath string) error {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return fmt.Errorf("не удалось открыть token бд: %w", err)
	}
	defer db.Close()

	createTableSQL := `
	CREATE TABLE IF NOT EXISTS refresh_tokens (
		rowID INTEGER PRIMARY KEY AUTOINCREMENT, 
		userID INTEGER,
		deviceName TEXT,
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

func (db *AppDB) InsertRefreshToken(userID int, deviceName string, refreshToken string, exp int64, iat int64, isRevoked bool) error {
	data := []byte(refreshToken)
	hash := sha256.Sum256(data)
	hashString := hex.EncodeToString(hash[:])
	query := `INSERT INTO refresh_tokens (userID, deviceName,tokenHash, exp, iat, isRevoked) VALUES (?, ?, ?, ?, ?, ?)`
	_, err := db.Exec(query, userID, deviceName, hashString, exp, iat, convertBoolToInt(isRevoked))
	return err
}

func (db *AppDB) GetRefreshTokenInfo(refreshToken string) (TokenData, error) {
	data := []byte(refreshToken)
	hash := sha256.Sum256(data)
	hashString := hex.EncodeToString(hash[:])
	query := `SELECT rowID, userID, deviceName, tokenHash, exp, iat, isRevoked FROM refresh_tokens WHERE tokenHash = ?`
	var token TokenData
	var rowID int
	err := db.QueryRow(query, hashString).Scan(
		&rowID,
		&token.UserID,
		&token.DeviceName,
		&token.RefreshToken,
		&token.Exp,
		&token.Iat,
		&token.IsRevoked)
	if err != nil {
		//if errors.Is(err, sql.ErrNoRows) {
		//	return TokenData{}, ErrTokenNotFound
		//}
		return TokenData{}, err
	}
	if expired(token.Exp) {
		query := `UPDATE refresh_tokens SET isRevoked  = 1 WHERE rowID = ?`
		_, err := db.Exec(query, rowID)
		if err != nil {
			return TokenData{}, err
		}
	}
	return token, nil
}

func expired(exp int64) bool {
	return exp < time.Now().Unix()
}
