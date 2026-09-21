package main

import (
	"ImageCacheProject/internal/brocker"
	"ImageCacheProject/internal/caching"
	"ImageCacheProject/internal/db"
	"ImageCacheProject/internal/request"
	"ImageCacheProject/internal/upload"
	"ImageCacheProject/internal/util"
	"context"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		slog.Error(".env file was not found")
	}

	coldBoot := flag.Bool("coldboot", false, "start DB")
	flag.Parse()
	if *coldBoot {
		db.ColdBoot()
	}

	setUpLogger()

	ctx, cancel, cancelChan := mainSetup()

	ldb, err := db.Connect()
	if err != nil {
		fmt.Print("main: ", err.Error())
		return
	}
	defer func() {
		err = ldb.Close()
		if err != nil {
			fmt.Print("main: ", err.Error())
		}
	}()

	cacheDir := os.Getenv("CACHE_DIR")
	cacheLibDir := os.Getenv("CACHE_LIB_DIR")
	cacheManDir := os.Getenv("CACHE_MAN_DIR")
	cacheModDir := os.Getenv("CACHE_MOD_DIR")
	uploadsDir := os.Getenv("UPLOADS_DIR")
	paths := util.Paths{OriginalsDir: uploadsDir, CacheDir: cacheDir, CacheModalDir: cacheModDir, CacheLibDir: cacheLibDir, CacheManDir: cacheManDir}

	cacheManager := caching.InitCache(ctx, &paths)
	uploadManager := upload.NewManager(&paths)

	if err != nil {
		panic(err)
	}
	cacheManager.StartBGProcesses()
	issChan := make(chan util.Issue, 10)
	errH := brocker.NewErrorHandler(ctx, &paths, issChan)
	go errH.RunHandler()
	dbh := brocker.NewDBHandler(ctx, issChan)
	dbh.InitDBHandler()
	wp := &db.WPSubDB{ldb}
	notifier := request.NewNotifier(wp)

	dbh.Subscribe(brocker.SendMessage, notifier)
	dbModule := createDBModule(ldb, dbh)

	signer := request.NewSigner()
	handler := request.NewHandler(ctx, cacheManager, uploadManager, dbModule, dbh, signer)

	srv := &http.Server{
		Addr: ":8080",
	}
	request.StartReqHandling(srv, handler, notifier)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-quit:

	case <-cancelChan:

	}
	cancel()
	cacheManager.Close()
	StopServer(srv)
}

func StopServer(srv *http.Server) {

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("error while shutting down server: %v", err)
	}
}

func setUpLogger() {
	file, err := os.OpenFile("app.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		panic("не удалось открыть файл логов: " + err.Error())
	}
	defer file.Close()

	logger := slog.New(slog.NewJSONHandler(file, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	slog.SetDefault(logger)

}

func mainSetup() (context.Context, context.CancelFunc, chan struct{}) {
	ctx, cancel := context.WithCancel(context.Background())
	cancelChan := make(chan struct{})
	return ctx, cancel, cancelChan
}

func createDBModule(appDB *db.AppDB, handler *brocker.Handler) *request.DBModule {
	ImageDB := &db.ImageDB{appDB}
	GroupDB := &db.GroupDB{appDB}
	GroupMessageDB := &db.MessageDB{appDB}
	ImageUserDB := &db.ImageUserDB{ImageDB}
	ImageGroupDB := &db.ImageGroupDB{ImageDB}
	TokenDB := &db.TokenDB{appDB}
	UserDB := &db.UserDB{appDB}
	handler.Subscribe(brocker.InsertNewToken, TokenDB)
	handler.Subscribe(brocker.RevokeToken, TokenDB)
	handler.Subscribe(brocker.InsertNewImage, ImageDB)
	handler.Subscribe(brocker.RemoveImage, ImageDB)
	handler.Subscribe(brocker.AddUserToGroup, GroupDB)
	handler.Subscribe(brocker.RemoveUserFromGroup, GroupDB)
	handler.Subscribe(brocker.AddMessage, GroupMessageDB)
	handler.Subscribe(brocker.ChangeMessageStatus, GroupMessageDB)
	handler.Subscribe(brocker.GroupServiceMessage, GroupDB)
	return &request.DBModule{
		GroupDB:        GroupDB,
		GroupMessageDB: GroupMessageDB,
		ImageUserDB:    ImageUserDB,
		ImageGroupDB:   ImageGroupDB,
		TokenDB:        TokenDB,
		UserDB:         UserDB,
	}
}
