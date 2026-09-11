package db

import (
	"ImageCacheProject/internal/brocker"
	"ImageCacheProject/internal/util"
	"database/sql"
	"fmt"
	"time"
)

type MessageDB struct {
	*AppDB
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
func (db *MessageDB) AddNewMessage(m util.Message) (int, error) {
	query := `INSERT INTO messages (senderID, recipientID, groupID, msgType, content, createdAt, status) VALUES ($1, $2, $3, $4, $5, $6, $7)`
	res, err := db.Exec(query, m.SenderID, m.RecipientID, m.GroupID, m.MsgType, m.Content, m.CreatedAt, m.Status)
	if err != nil {
		return -1, err
	}
	groupID, err := res.LastInsertId()
	if err != nil {
		return -1, fmt.Errorf("unable to get groupID %w", err)
	}
	return int(groupID), nil
}

func FormNewMessage(senderID int, recipientID int, groupID int, msgType util.MsgType, content string) util.Message {
	createdAt := time.Now().Unix()
	status := util.Acquired
	return util.Message{SenderID: senderID, RecipientID: recipientID, GroupID: groupID, MsgType: msgType, Content: content, CreatedAt: createdAt, Status: status}
}

func (db *MessageDB) changeMsgStatus(messageID int, status util.MsgStatus) error {
	query := `UPDATE messages SET status = ?  WHERE messageID = ?`
	_, err := db.Exec(query, status, messageID)
	return err
}

func (db *MessageDB) ProcessEvent(e brocker.Event) util.Issue {
	datablob := e.GetData()
	if datablob == nil {
		return &ErrIssue{brocker.ErrNoDataInEvent, ""}
	}
	switch e.GetTopic() {
	case brocker.ChangeMessageStatus:
		{
			data, ok := datablob.(util.Message)
			if !ok {
				return &ErrIssue{err: ErrConversion, desc: fmt.Sprintf("unable to convert %s to UserGroup", e.GetData())}
			}
			err := db.changeMsgStatus(data.MessageID, data.Status)
			if err != nil {
				return &ErrIssue{err: err, desc: "unable to change message status"}
			}
		}
	default:
		return &ErrIssue{
			err: ErrWrongTopic,
			desc: fmt.Sprintf("sent topic: %d, expected %d or %d",
				e.GetTopic(), brocker.AddUserToGroup, brocker.RemoveUserFromGroup)}
	}
	return nil
}
