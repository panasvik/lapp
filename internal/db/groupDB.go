package db

import (
	"ImageCacheProject/internal/brocker"
	"ImageCacheProject/internal/util"
	"database/sql"
	"errors"
	"fmt"
)

var (
	ErrGroupAlreadyExists = errors.New("group already exists")
)

type GroupDB struct {
	*AppDB
}

func initGroupDB(dbPath string) error {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return fmt.Errorf("не удалось открыть user бд: %w", err)
	}
	defer db.Close()

	createGroupsTableSQL := `
	CREATE TABLE IF NOT EXISTS groups (
		groupID INTEGER PRIMARY KEY AUTOINCREMENT,
	    groupName TEXT, 
		creatorID INTEGER,
	);`
	createGroupUsersTableSQL := `
	CREATE TABLE IF NOT EXISTS groupUsers (
		rowID INTEGER PRIMARY KEY AUTOINCREMENT,	    
		groupID INTEGER,
		userID INTEGER,
		role TEXT
	);`
	if _, err := db.Exec(createGroupsTableSQL); err != nil {
		return fmt.Errorf("unable to create groups table: %w", err)
	}
	if _, err := db.Exec(createGroupUsersTableSQL); err != nil {
		return fmt.Errorf("unable to create group users table: %w", err)
	}
	return nil
}

func (db *GroupDB) RegisterNewGroup(groupName string, creatorID int) (int, error) {
	if db.groupExists(groupName) {
		return -1, fmt.Errorf("%w by the name of %s", ErrGroupAlreadyExists, groupName)
	}
	query := `INSERT INTO groups (groupName, creatorID) VALUES ($1, $2)`
	res, err := db.Exec(query, groupName, creatorID)
	if err != nil {
		return -1, err
	}
	groupID, err := res.LastInsertId()
	if err != nil {
		return -1, fmt.Errorf("unable to get groupID %w", err)
	}
	return int(groupID), nil
}

func (db *GroupDB) GetGroupUsersID(groupID int) ([]int, error) {
	query := `SELECT userID FROM groupUsers WHERE groupID = ?`
	rows, err := db.Query(query, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var userIDs []int
	var userID int
	for rows.Next() {
		if err := rows.Scan(&userID); err != nil {
			return nil, fmt.Errorf("error parsing string: %w", err)
		}
		userIDs = append(userIDs, userID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error reading results: %w", err)
	}
	return userIDs, nil
}

func (db *GroupDB) addUserToGroup(groupID int, userID int, role string) error {
	query := `INSERT INTO groupUsers (groupID, userID, role) VALUES ($1, $2, $3)`
	_, err := db.Exec(query, groupID, userID, role)
	return err
}

func (db *GroupDB) removeUserFromGroup(groupID int, userID int) error {
	query := `DELETE FROM groupUsers WHERE groupID = ? and userID = ?`
	_, err := db.Exec(query, groupID, userID)
	return err
}

func (db *GroupDB) groupExists(groupName string) bool {
	query := `SELECT groupID FROM groups WHERE groupName = ?`
	var groupID int
	err := db.QueryRow(query, groupName).Scan(&groupID)
	return err == nil
}

func (db *GroupDB) ProcessEvent(e brocker.Event) util.Issue {
	datablob := e.GetData()
	if datablob == nil {
		return &ErrIssue{brocker.ErrNoDataInEvent, ""}
	}
	switch e.GetTopic() {
	case brocker.AddUserToGroup:
		{
			data, ok := datablob.(util.UserGroup)
			if !ok {
				return &ErrIssue{err: ErrConversion, desc: fmt.Sprintf("unable to convert %s to UserGroup", e.GetData())}
			}
			err := db.addUserToGroup(data.GroupID, data.UserID, data.Role)
			if err != nil {
				return &ErrIssue{err: err, desc: ""}
			}
		}
	case brocker.RemoveUserFromGroup:
		{
			data, ok := datablob.(util.UserGroup)
			if !ok {
				return &ErrIssue{err: ErrConversion, desc: fmt.Sprintf("unable to convert %s to UserGroup", e.GetData())}
			}
			err := db.removeUserFromGroup(data.GroupID, data.UserID)
			if err != nil {
				return &ErrIssue{err: err, desc: ""}
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
