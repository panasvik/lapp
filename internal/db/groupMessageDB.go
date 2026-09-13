package db

import (
	"ImageCacheProject/internal/brocker"
	"ImageCacheProject/internal/util"
	"database/sql"
	"fmt"
)

type MessageDB struct {
	*AppDB
}

func initMessagesDB(dbPath string) error {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return fmt.Errorf("не удалось открыть message бд: %w", err)
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
	query := `INSERT INTO messages (senderID, recipientID, groupID, msgType, content, createdAt, status) VALUES (?, ?, ?, ?, ?, ?, ?)`
	res, err := db.Exec(query, m.SenderID, m.RecipientID, m.GroupID, m.MsgType, m.Content, m.CreatedAt, m.Status)
	if err != nil {
		return -1, err
	}
	messageID, err := res.LastInsertId()
	if err != nil {
		return -1, fmt.Errorf("unable to get messageID %w", err)
	}
	return int(messageID), nil
}

func (db *MessageDB) GetUnreadMessages(recipientID int) ([]util.Message, error) {
	query := `SELECT * from messages WHERE recipientID = ? AND Status < 2`
	rows, err := db.Query(query, recipientID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var messages []util.Message
	var rowErr error
	for rows.Next() {
		var message util.Message
		rowErr = rows.Scan(&message.MessageID,
			&message.SenderID,
			&message.RecipientID,
			&message.GroupID,
			&message.MsgType,
			&message.Content,
			&message.CreatedAt,
			&message.Status)
		if rowErr != nil {
			err = rowErr
			continue
		}
		messages = append(messages, message)
	}
	return messages, err
}

func (db *MessageDB) changeMsgStatus(messageID int, status util.MsgStatus) error {
	query := `UPDATE messages SET status = ?  WHERE messageID = ?`
	_, err := db.Exec(query, status, messageID)
	return err
}

func (db *MessageDB) ProcessEvent(e brocker.Event) util.Issue {
	datablob := e.Body
	if datablob == nil {
		return &util.ErrIssue{brocker.ErrNoDataInEvent, ""}
	}
	switch e.Topic {
	case brocker.AddMessage:
		{
			data, ok := datablob.(util.Message)
			if !ok {
				return &util.ErrIssue{Err: brocker.ErrConversion, Desc: fmt.Sprintf("unable to convert %s to UserGroup", e.Body)}
			}
			msgID, err := db.AddNewMessage(data)
			if err != nil {
				return &util.ErrIssue{Err: err, Desc: "unable to add message"}
			}
			data.MessageID = msgID
			e.Forward(data, brocker.SendMessage)
		}
	case brocker.ChangeMessageStatus:
		{
			data, ok := datablob.(util.Message)
			if !ok {
				return &util.ErrIssue{Err: brocker.ErrConversion, Desc: fmt.Sprintf("unable to convert %s to UserGroup", e.Body)}
			}
			err := db.changeMsgStatus(data.MessageID, data.Status)
			if err != nil {
				return &util.ErrIssue{Err: err, Desc: "unable to change message status"}
			}
		}

	default:
		return &util.ErrIssue{
			Err: brocker.ErrWrongTopic,
			Desc: fmt.Sprintf("sent topic: %d, expected %d or %d",
				e.Topic, brocker.AddMessage, brocker.ChangeMessageStatus)}
	}
	return nil
}
