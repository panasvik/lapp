package db

import (
	"database/sql"
	"fmt"
	"time"
)

type MsgType int

const (
	Invitation MsgType = 0
	Rejection  MsgType = 1
	Consent    MsgType = 2
)

type MsgStatus int

const (
	Acquired  MsgStatus = 0
	Sent      MsgStatus = 1
	Delivered MsgStatus = 2
	Read      MsgStatus = 3
)

type Message struct {
	messageID   int
	senderID    int
	recipientID int
	groupID     int
	msgType     MsgType
	content     string
	createdAt   int64 //Unix timestamp
	status      MsgStatus
}

func initMessagesDB(dbPath string) error {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return fmt.Errorf("не удалось открыть user бд: %w", err)
	}
	defer db.Close()

	createTableSQL := `
	CREATE TABLE IF NOT EXISTS messages (
		messageID INTEGER PRIMARY KEY AUTOINCREMENT,
	    senderID INTEGER, 
		recipientID INTEGER,
		groupID INTEGER,
		msgType INTEGER,
		content TEXT,
		createdAt INTEGER,
	    status INTEGER
	);`
	if _, err := db.Exec(createTableSQL); err != nil {
		return fmt.Errorf("unable to create user table: %w", err)
	}
	return nil
}

// addNewMessage: m.messageID is ignored when adding a message
func (db *AppDB) addNewMessage(m Message) (int, error) {
	query := `INSERT INTO messages (senderID, recipientID, groupID, msgType, content, createdAt, status) VALUES ($1, $2, $3, $4, $5, $6, $7)`
	res, err := db.Exec(query, m.senderID, m.recipientID, m.groupID, m.msgType, m.content, m.createdAt, m.status)
	if err != nil {
		return -1, err
	}
	groupID, err := res.LastInsertId()
	if err != nil {
		return -1, fmt.Errorf("unable to get groupID %w", err)
	}
	return int(groupID), nil
}

func FormNewMessage(senderID int, recipientID int, groupID int, msgType MsgType, content string) Message {
	createdAt := time.Now().Unix()
	status := Acquired
	return Message{senderID: senderID, recipientID: recipientID, groupID: groupID, msgType: msgType, content: content, createdAt: createdAt, status: status}
}

func (db *AppDB) changeMsgStatus(messageID int, status MsgStatus) error {
	query := `UPDATE messages SET status = ?  WHERE messageID = ?`
	_, err := db.Exec(query, status, messageID)
	return err
}
