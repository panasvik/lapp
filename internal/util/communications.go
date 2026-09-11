package util

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
	MessageID   int
	SenderID    int
	RecipientID int
	GroupID     int
	MsgType     MsgType
	Content     string
	CreatedAt   int64 //Unix timestamp
	Status      MsgStatus
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
