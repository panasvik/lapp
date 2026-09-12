package util

import (
	"fmt"
	"time"
)

type MsgType int

const (
	Invitation MsgType = 0
	Rejection  MsgType = 1
	Consent    MsgType = 2
	LibUpdate  MsgType = 3
)

type MsgStatus int

const (
	Acquired  MsgStatus = 0
	Sent      MsgStatus = 1
	Delivered MsgStatus = 2
	Read      MsgStatus = 3
)

type Message struct {
	MessageID   int       `json:"msgID"`
	SenderID    int       `json:"senderID"`
	RecipientID int       `json:"recipientID"`
	GroupID     int       `json:"groupID"`
	MsgType     MsgType   `json:"MsgType"`
	Content     string    `json:"content"`
	CreatedAt   int64     `json:"CreatedAt"` //Unix timestamp
	Status      MsgStatus `json:"status"`
}

func FormNewMessage(senderID int, recipientID int, groupID int, msgType MsgType, content string) Message {
	createdAt := time.Now().Unix()
	status := Acquired
	return Message{SenderID: senderID, RecipientID: recipientID, GroupID: groupID, MsgType: msgType, Content: content, CreatedAt: createdAt, Status: status}
}

type ImageHolderType int

const (
	User  = ImageHolderType(0)
	Group = ImageHolderType(1)
)

type ImgData struct {
	HolderID int
	Holder   ImageHolderType
	Path     string
	Date     int
	Callback func() error
}

type TokenData struct {
	UserID       int
	DeviceName   string
	RefreshToken string
	Exp          int64
	Iat          int64
	IsRevoked    bool
}

type UserLogOut struct {
	UserID     int
	DeviceName string
}

type UserGroup struct {
	UserID  int
	GroupID int
	Role    string
}

type UserDevice struct {
	UserID int
	Device string
	OS     string
}

func (t *MsgType) Scan(value interface{}) error {
	if value == nil {
		*t = 0
		return nil
	}

	if i, ok := value.(int64); ok {
		*t = MsgType(i)
		return nil
	}
	return fmt.Errorf("cannot scan %T into topic", value)
}

func (s *MsgStatus) Scan(value interface{}) error {
	if value == nil {
		*s = 0
		return nil
	}

	if i, ok := value.(int64); ok {
		*s = MsgStatus(i)
		return nil
	}
	return fmt.Errorf("cannot scan %T into topic", value)
}
