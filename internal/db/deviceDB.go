package db

import (
	"ImageCacheProject/internal/brocker"
	"ImageCacheProject/internal/util"
	"database/sql"
	"fmt"
)

type DeviceDB struct {
	*AppDB
}

func initDeviceTable(dbPath string) error {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return fmt.Errorf("не удалось открыть token бд: %w", err)
	}
	defer db.Close()

	createTableSQL := `
	CREATE TABLE IF NOT EXISTS devices (
		rowID INTEGER PRIMARY KEY AUTOINCREMENT, 
		userID INTEGER,
		deviceName TEXT,
	    os TEXT
	);`
	if _, err := db.Exec(createTableSQL); err != nil {
		return fmt.Errorf("unable to create a table: %w", err)
	}
	return nil
}

func (db *DeviceDB) InsertNewDevice(userID int, deviceName string, os string) (int, error) {
	query := `INSERT INTO devices (userID, deviceName, os) VALUES (?, ?, ?)`
	res, err := db.Exec(query, userID, deviceName, os)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	return int(id), nil
}

func (db *DeviceDB) GetDevicesTokenInfo(userID int) ([]util.UserDevice, error) {
	query := `SELECT userID, deviceName, os FROM devices WHERE userID = ?`
	rows, err := db.Query(query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var devices []util.UserDevice
	for rows.Next() {
		var device util.UserDevice
		if err := rows.Scan(&device.UserID, &device.Device, &device.OS); err != nil {
			return nil, fmt.Errorf("ошибка сканирования строки: %w", err)
		}
		devices = append(devices, device)
	}
	return devices, nil

}

func (db *DeviceDB) ProcessEvent(e brocker.Event) util.Issue {
	data := e.Body
	switch e.Topic {
	case brocker.InsertNewDevice:
		deviceD, ok := data.(util.UserDevice)
		if !ok {
			return &util.ErrIssue{Err: brocker.ErrConversion, Desc: fmt.Sprintf("unable to convert %s to TokenData", e.Body)}
		}
		_, err := db.InsertNewDevice(deviceD.UserID, deviceD.Device, deviceD.OS)
		if err != nil {
			return &util.ErrIssue{err, "unable to insert refresh token"}
		}
	default:
		return &util.ErrIssue{
			Err: brocker.ErrWrongTopic,
			Desc: fmt.Sprintf("sent topic: %d, expected %d",
				e.Topic, brocker.InsertNewDevice)}
	}
	return nil
}
