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

type TokenDB struct {
	*AppDB
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

func (db *TokenDB) InsertRefreshToken(userID int, deviceName string, refreshToken string, exp int64, iat int64, isRevoked bool) error {
	data := []byte(refreshToken)
	hash := sha256.Sum256(data)
	hashString := hex.EncodeToString(hash[:])
	query := `INSERT INTO refresh_tokens (userID, deviceName,tokenHash, exp, iat, isRevoked) VALUES (?, ?, ?, ?, ?, ?)`
	_, err := db.Exec(query, userID, deviceName, hashString, exp, iat, util.ConvertBoolToInt(isRevoked))
	return err
}

func (db *TokenDB) GetRefreshTokenInfo(refreshToken string) (util.TokenData, error) {
	data := []byte(refreshToken)
	hash := sha256.Sum256(data)
	hashString := hex.EncodeToString(hash[:])
	query := `SELECT rowID, userID, deviceName, tokenHash, exp, iat, isRevoked FROM refresh_tokens WHERE tokenHash = ?`
	var token util.TokenData
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
		return util.TokenData{}, err
	}
	if expired(token.Exp) {
		query := `UPDATE refresh_tokens SET isRevoked  = 1 WHERE rowID = ?`
		_, err := db.Exec(query, rowID)
		if err != nil {
			return util.TokenData{}, err
		}
	}
	return token, nil
}

func expired(exp int64) bool {
	return exp < time.Now().Unix()
}

func (db *TokenDB) LogOutUser(userID int, deviceName string) error {
	query := `UPDATE refresh_tokens SET isRevoked  = 1 WHERE userID = ? AND deviceName = ?`
	_, err := db.Exec(query, userID, deviceName)
	return err
}

func (db *TokenDB) ProcessEvent(e brocker.Event) util.Issue {
	data := e.GetData()
	switch e.GetTopic() {
	case brocker.InsertNewToken:
		tokenD, ok := data.(util.TokenData)
		if !ok {
			return &ErrIssue{err: ErrConversion, desc: fmt.Sprintf("unable to convert %s to TokenData", e.GetData())}
		}
		err := db.InsertRefreshToken(tokenD.UserID, tokenD.DeviceName, tokenD.RefreshToken, tokenD.Exp, tokenD.Iat, tokenD.IsRevoked)
		if err != nil {
			return &ErrIssue{err, "unable to insert refresh token"}
		}
	case brocker.RevokeToken:
		logout, ok := data.(util.UserLogOut)
		if !ok {
			return &ErrIssue{err: ErrConversion, desc: fmt.Sprintf("unable to convert %s to UserLogOut", e.GetData())}
		}
		err := db.LogOutUser(logout.UserID, logout.DeviceName)
		if err != nil {
			return &ErrIssue{err, "unable to make logout changes in db"}
		}
	default:
		return &ErrIssue{
			err: ErrWrongTopic,
			desc: fmt.Sprintf("sent topic: %d, expected %d or %d",
				e.GetTopic(), brocker.InsertNewToken, brocker.RevokeToken)}
	}
	return nil
}
