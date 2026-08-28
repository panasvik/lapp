package main

import (
	"ImageCacheProject/internal/caching"
	"ImageCacheProject/internal/db"
	"ImageCacheProject/internal/env"
	"ImageCacheProject/internal/request"
	"bufio"
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
)

func main() {
	coldBoot := flag.Bool("coldboot", false, "start DB")
	flag.Parse()
	if *coldBoot {
		db.ColdBoot()
	}

	setUpLogger()

	ctx, cancel := context.WithTimeout(context.Background(), time.Hour)
	scanner := bufio.NewScanner(os.Stdin)
	cancelChan := make(chan struct{})
	go readInput(cancelChan, scanner)

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

	eem := env.NewEventManager()
	fmu := caching.NewTable()
	cacheManager := caching.InitCache(ctx, fmu, eem)
	err = eem.InitPaths(".")
	if err != nil {
		panic(err)
	}
	cacheManager.StartBGProcesses()

	handler := request.NewHandler(ctx, ldb, cacheManager, fmu)
	srv := &http.Server{
		Addr: ":8080",
	}
	request.StartReqHandling(srv, handler)

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

func readInput(cancelChan chan<- struct{}, scanner *bufio.Scanner) {
	for scanner.Scan() {
		line := scanner.Text()
		if line == "exit" {
			cancelChan <- struct{}{}
			break
		}
	}
}
