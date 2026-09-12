package db

import (
	"database/sql"
	"fmt"

	webpush "github.com/SherClockHolmes/webpush-go"
)

type WPSubDB struct {
	*AppDB
}

func initWBSubDB(dbPath string) error {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return fmt.Errorf("не удалось открыть WPSubDB бд: %w", err)
	}
	defer db.Close()
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS webPushSubs (
		subID INTEGER PRIMARY KEY AUTOINCREMENT,
		userID INTEGER,
		endpoint TEXT,
		p256dh TEXT,
		auth TEXT
	);`
	if _, err := db.Exec(createTableSQL); err != nil {
		return fmt.Errorf("unable to create user table: %w", err)
	}
	createIndexSQL := `
    CREATE UNIQUE INDEX IF NOT EXISTS idxWpsubsUser
    ON webPushSubs (userID, endpoint);`

	if _, err := db.Exec(createIndexSQL); err != nil {
		return fmt.Errorf("не удалось создать уникальный индекс: %w", err)
	}
	return nil
}

func (db *WPSubDB) GetSubs(userID int) ([]webpush.Subscription, error) {
	query := `SELECT (endpoint, p256dh, auth) FROM webPushSubs WHERE userID = ?`
	rows, err := db.Query(query, userID)
	if err != nil {
		return nil, fmt.Errorf("unable to get subs %w", err)
	}
	defer rows.Close()

	var subs []webpush.Subscription

	for rows.Next() {
		var sub webpush.Subscription
		if err := rows.Scan(&sub.Endpoint, &sub.Keys.P256dh, &sub.Keys.Auth); err != nil {
			return nil, fmt.Errorf("ошибка сканирования строки: %w", err)
		}
		subs = append(subs, sub)
	}
	return subs, nil
}

func (db *WPSubDB) AddSub(userID int, sub webpush.Subscription) (int, error) {
	query := `INSERT INTO webPushSubs (userID, endpoint, p256dh, auth) 
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (userID, endpoint) DO NOTHING`
	res, err := db.Exec(query, userID, sub.Endpoint, sub.Keys.P256dh, sub.Keys.Auth)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	return int(id), nil
}

func (db *WPSubDB) RemoveSub(userID int, sub webpush.Subscription) error {
	query := `DELETE FROM webPushSubs WHERE userID = ? AND endpoint = ?`
	_, err := db.Exec(query, userID, sub.Endpoint)
	return err
}
