package request

import (
	"ImageCacheProject/internal/util"

	"github.com/SherClockHolmes/webpush-go"
)

type DBModule struct {
	GroupDB
	GroupMessageDB
	ImageUserDB
	ImageGroupDB
	TokenDB
	UserDB
}

type GroupDB interface {
	RegisterNewGroup(groupName string, creatorID int) (int, error)
	GetGroupUserIDs(groupID int) ([]int, error)
	GetUserGroupNames(userID int) ([]string, error)
	GetUserGroupIDs(userID int) ([]int, error)
	GetGroupName(groupID int) (string, error)
}

type GroupMessageDB interface {
	AddNewMessage(m util.Message) (int, error)
	GetUnreadMessages(recipientID int) ([]util.Message, error)
}

type ImageGroupDB interface {
	GetNamesGroup(targetDate int, groupID int) (names []string, err error)
	GetDatesGroup(groupID int) (dates []int, err error)
	GetLibsNamesGroup(groupID int) (names []string, err error)
	ImageBelongsToGroup(path string, groupID int) (bool, error)
}

type ImageUserDB interface {
	GetNamesUser(targetDate int, userID int) (names []string, err error)
	GetDatesUser(userID int) (dates []int, err error)
	GetLibsNamesUser(userID int) (names []string, err error)
	ImageBelongsToUser(path string, userID int) (bool, error)
}

type TokenDB interface {
	GetRefreshTokenInfo(refreshToken string) (util.TokenData, error)
}

type UserDB interface {
	RegisterNewUser(userName string, password string) (int, error)
	GetUserIDByCredentials(userName string, password string) (int, error)
	GetUserNameByID(userID int) (string, error)
}

type WebPushDB interface {
	AddSub(userID int, sub webpush.Subscription) (int, error)
	GetSubs(userID int) ([]webpush.Subscription, error)
	RemoveSub(userID int, sub webpush.Subscription) error
}

type DeviceDB interface {
	GetDevicesTokenInfo(userID int) ([]util.UserDevice, error)
	InsertNewDevice(userID int, deviceName string, os string) (int, error)
}
