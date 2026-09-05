package db

import (
	"ImageCacheProject/assets"
	"ImageCacheProject/internal/brocker"
	"ImageCacheProject/internal/util"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log"
	"sync/atomic"
	"time"
)

const (
	maxDBreq = 10
)

type AppDB struct {
	*sql.DB
	IsOpen atomic.Bool
	limit  chan struct{}
}

func Connect() (*AppDB, error) {
	db, err := sql.Open("sqlite3", "loveApp.db")
	if err != nil {
		return nil, err
	}
	d := &AppDB{
		DB: db, limit: make(chan struct{}, maxDBReq)}
	d.IsOpen.Store(true)
	return d, nil
}

func (db *AppDB) fastGetNames(targetDate int, userID int) (names []string, err error) {
	query := `
		SELECT img_path
		FROM images
		WHERE userID = ? AND date = ?`
	rows, err := db.Query(query, userID, targetDate)

	if err != nil {
		return nil, fmt.Errorf("ошибка выполнения запроса: %w", err)
	}

	defer rows.Close()

	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("ошибка сканирования строки: %w", err)
		}
		names = append(names, name)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ошибка при чтении результатов: %w", err)
	}
	if len(names) == 0 {
		return nil, ErrNoNames
	}
	return names, nil
}

func (db *AppDB) GetNames(targetDate int, userID int) (names []string, err error) {
	if !db.IsOpen.Load() {
		return nil, fmt.Errorf("DataBase is closed")
	}

	names, err = db.fastGetNames(targetDate, userID)
	if err == nil {
		return names, nil
	}

	query := `
		SELECT img_path 
		FROM images 
		WHERE userID = ? AND date = (
			SELECT date 
			FROM images 
			WHERE userID = ? 
			ORDER BY ABS(date - ?) ASC 
			LIMIT 1
		)
	`

	rows, err := db.Query(query, userID, userID, targetDate)
	if err != nil {
		return nil, fmt.Errorf("error completing request: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("error parsing string: %w", err)
		}
		names = append(names, name)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error reading results: %w", err)
	}

	return names, nil
}

func (db *AppDB) GetLibsNames(userID int) (names []string, err error) {
	if !db.IsOpen.Load() {
		return nil, fmt.Errorf("DataBase is closed")
	}

	query := `
		SELECT img_path 
		FROM images 
		WHERE userID = ? 
	`

	rows, err := db.Query(query, userID)
	if err != nil {
		return nil, fmt.Errorf("error completing request: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("error parsing string: %w", err)
		}
		names = append(names, name)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error reading results: %w", err)
	}

	return names, nil
}

func ColdBoot() {
	err := initAndPopulateImageDB("loveApp.db")
	if err != nil {
		log.Fatalf("Ошибка при работе с БД: %v", err)
	}
	err = initTokenTable("loveApp.db")
	if err != nil {
		log.Fatalf("Ошибка при работе с БД: %v", err)
	}
	err = initUserDB("loveApp.db")
	if err != nil {
		log.Fatalf("Ошибка при работе с БД: %v", err)
	}
	fmt.Println("База данных успешно создана и заполнена!")
}

func initAndPopulateImageDB(dbPath string) error {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return fmt.Errorf("не удалось открыть image бд: %w", err)
	}
	defer db.Close()

	createTableSQL := `
    CREATE TABLE IF NOT EXISTS images (
       userID INTEGER,
       img_path TEXT,
       date INTEGER
    );`

	if _, err := db.Exec(createTableSQL); err != nil {
		return fmt.Errorf("не удалось создать таблицу: %w", err)
	}

	createIndexSQL := `
    CREATE UNIQUE INDEX IF NOT EXISTS idx_images_user_path 
    ON images (userID, img_path);`

	if _, err := db.Exec(createIndexSQL); err != nil {
		return fmt.Errorf("не удалось создать уникальный индекс: %w", err)
	}

	query := `
    INSERT INTO images (userID, img_path, date) 
    VALUES (?, ?, ?)
    ON CONFLICT (userID, img_path) DO NOTHING`

	stmt, err := db.Prepare(query)
	if err != nil {
		return fmt.Errorf("ошибка подготовки запроса: %w", err)
	}
	defer stmt.Close()

	data := make(chan assets.RowData, 5)
	errChan := make(chan error)
	go assets.PopulateDB(data, errChan)
	for item := range data {
		_, err = stmt.Exec(item.UserID, item.Name, item.Date)
		if err != nil {
			log.Printf("Предупреждение: не удалось добавить файл %s в БД: %v", item.Name, err)
		}
	}
	return <-errChan
}

func (db *AppDB) InsertImage(path string, userID int, date int) error {
	query := `
    	INSERT INTO images (userID, img_path, date) 
    	VALUES (?, ?, ?) 
    	ON CONFLICT (userID, img_path) DO NOTHING
	`

	_, err := db.Exec(query, userID, path, date)
	return err
}

func (db *AppDB) GetUserIDsByImgName(path string) ([]int, error) {
	query := `SELECT userID FROM images WHERE img_path = ?`
	rows, err := db.Query(query, path)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var userIDs []int
	for rows.Next() {
		var userID int
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

type ErrIssue struct {
	err  error
	desc string
}

func (e *ErrIssue) GetErr() error {
	return e.err
}

func (e *ErrIssue) GetBody() string {
	return e.desc
}

func (e *ErrIssue) GetFixCallback() func() error {
	return nil
}

type FileRemove struct {
	err      error
	fileName string
	callback func() error
}

func (f *FileRemove) GetErr() error {
	return f.err
}

func (f *FileRemove) GetBody() string {
	return f.fileName
}

func (f *FileRemove) GetFixCallback() func() error {
	return f.callback
}

func convertBoolToInt(b bool) int {
	switch b {
	case true:
		return 1
	case false:
		return 0
	default:
		return -1
	}
}

func initTokenTable(dbPath string) error {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return fmt.Errorf("не удалось открыть token бд: %w", err)
	}
	defer db.Close()

	createTableSQL := `
	CREATE TABLE IF NOT EXISTS refresh_tokens (
		rowID INTEGER PRIMARY KEY AUTOINCREMENT, 
		userID INTEGER,
		deviceName TEXT,
	    tokenHash TEXT, 
		exp INTEGER,
		iat INTEGER,
		isRevoked INTEGER
	);`
	if _, err := db.Exec(createTableSQL); err != nil {
		return fmt.Errorf("unable to create a table: %w", err)
	}
	return nil
}

func (db *AppDB) InsertRefreshToken(userID int, deviceName string, refreshToken string, exp int64, iat int64, isRevoked bool) error {
	data := []byte(refreshToken)
	hash := sha256.Sum256(data)
	hashString := hex.EncodeToString(hash[:])
	query := `INSERT INTO refresh_tokens (userID, deviceName,tokenHash, exp, iat, isRevoked) VALUES (?, ?, ?, ?, ?, ?)`
	_, err := db.Exec(query, userID, deviceName, hashString, exp, iat, convertBoolToInt(isRevoked))
	return err
}

func (db *AppDB) performDBTask(data any, topic brocker.DBTopic) util.Issue {
	switch topic {
	case brocker.InsertNewImage:
		imgd := data.(ImgData)
		err := db.InsertImage(imgd.Path, imgd.UserID, imgd.Date)
		if err != nil {
			return &FileRemove{ErrUpload, imgd.Path, imgd.callback}
		}
		return nil
	case brocker.InsertNewToken:
		userd := data.(TokenData)
		err := db.InsertRefreshToken(userd.UserID, userd.DeviceName, userd.RefreshToken, userd.Exp, userd.Iat, userd.IsRevoked)
		return &ErrIssue{err, ""}
	case brocker.RevokeToken:
		logout := data.(UserLogOut)
		err := db.LogOutUser(logout.UserID, logout.DeviceName)
		return &ErrIssue{err, ""}

	}
	return &ErrIssue{brocker.ErrUnknownDBTopic, "unable to form userData"}
}

func (db *AppDB) GetRefreshTokenInfo(refreshToken string) (TokenData, error) {
	data := []byte(refreshToken)
	hash := sha256.Sum256(data)
	hashString := hex.EncodeToString(hash[:])
	query := `SELECT rowID, userID, deviceName, tokenHash, exp, iat, isRevoked FROM refresh_tokens WHERE tokenHash = ?`
	var token TokenData
	var rowID int
	err := db.QueryRow(query, hashString).Scan(
		&rowID,
		&token.UserID,
		&token.DeviceName,
		&token.RefreshToken,
		&token.Exp,
		&token.Iat,
		&token.IsRevoked)
	if err != nil {
		//if errors.Is(err, sql.ErrNoRows) {
		//	return TokenData{}, ErrTokenNotFound
		//}
		return TokenData{}, err
	}
	if expired(token.Exp) {
		query := `UPDATE refresh_tokens SET isRevoked  = 1 WHERE rowID = ?`
		_, err := db.Exec(query, rowID)
		if err != nil {
			return TokenData{}, err
		}
	}
	return token, nil
}

func expired(exp int64) bool {
	return exp < time.Now().Unix()
}

func initUserDB(dbPath string) error {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return fmt.Errorf("не удалось открыть user бд: %w", err)
	}
	defer db.Close()

	createTableSQL := `
	CREATE TABLE IF NOT EXISTS users (
		userID INTEGER PRIMARY KEY AUTOINCREMENT,
	    userName TEXT, 
		passwordHash TEXT
	);`
	if _, err := db.Exec(createTableSQL); err != nil {
		return fmt.Errorf("unable to create user table: %w", err)
	}
	return nil
}
func (db *AppDB) RegisterNewUser(userName string, password string) (int, error) {
	if db.userExists(userName) {
		return -1, fmt.Errorf("%w by the name of %s", ErrUserAlreadyExists, userName)
	}
	data := []byte(password)
	hash := sha256.Sum256(data)
	hashString := hex.EncodeToString(hash[:])
	query := `INSERT INTO users (userName, passwordHash) VALUES (?, ?)`
	_, err := db.Exec(query, userName, hashString)
	if err != nil {
		return -1, err
	}
	return db.CheckUserPassword(userName, password)
}

func (db *AppDB) userExists(userName string) bool {
	query := `SELECT userID FROM users WHERE userName = ?`
	var userID int
	err := db.QueryRow(query, userName).Scan(&userID)
	return err == nil
}

func (db *AppDB) CheckUserPassword(userName string, password string) (int, error) {
	data := []byte(password)
	hash := sha256.Sum256(data)
	hashString := hex.EncodeToString(hash[:])
	query := `SELECT userID FROM users WHERE userName = ? AND passwordHash = ?`
	var userID int
	err := db.QueryRow(query, userName, hashString).Scan(&userID)
	return userID, err
}

func (db *AppDB) LogOutUser(userID int, deviceName string) error {
	query := `UPDATE refresh_tokens SET isRevoked  = 1 WHERE userID = ? AND deviceName = ?`
	_, err := db.Exec(query, userID, deviceName)
	return err
}

func (db *AppDB) PushLimit() {
	db.limit <- struct{}{}
}

func (db *AppDB) PullLimit() {
	<-db.limit
}

func (db *AppDB) ProcessEvent(e brocker.Event) util.Issue {
	data := e.GetData()
	if data == nil {
		return &ErrIssue{brocker.ErrNoDataInEvent, ""}
	}
	return db.performDBTask(data, e.GetTopic())
}
