package request

import (
	"ImageCacheProject/internal/brocker"
	"ImageCacheProject/internal/util"
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
	GetGroupUsersID(groupID int) ([]int, error)
	ProcessEvent(e brocker.Event) util.Issue
	PushLimit()
	PullLimit()
}

type GroupMessageDB interface {
	AddNewMessage(m util.Message) (int, error)
	ProcessEvent(e brocker.Event) util.Issue
	PushLimit()
	PullLimit()
}

type ImageGroupDB interface {
	GetNamesGroup(targetDate int, groupID int) (names []string, err error)
	GetDatesGroup(groupID int) (dates []int, err error)
	GetLibsNamesGroup(groupID int) (names []string, err error)
	NameBelongsToGroup(path string, groupID int) (bool, error)
	ProcessEvent(e brocker.Event) util.Issue
	PushLimit()
	PullLimit()
}

type ImageUserDB interface {
	GetNamesUser(targetDate int, userID int) (names []string, err error)
	GetDatesUser(userID int) (dates []int, err error)
	GetLibsNamesUser(userID int) (names []string, err error)
	NameBelongsToUser(path string, userID int) (bool, error)
	ProcessEvent(e brocker.Event) util.Issue
	PushLimit()
	PullLimit()
}

type TokenDB interface {
	GetRefreshTokenInfo(refreshToken string) (util.TokenData, error)
	ProcessEvent(e brocker.Event) util.Issue
	PushLimit()
	PullLimit()
}

type UserDB interface {
	RegisterNewUser(userName string, password string) (int, error)
	GetUserIDByCredentials(userName string, password string) (int, error)
	//ProcessEvent(e brocker.Event) util.Issue
	PushLimit()
	PullLimit()
}
