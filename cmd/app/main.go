package main

import (
	"ImageCacheProject/internal/caching"
	"ImageCacheProject/internal/db"
	"ImageCacheProject/internal/request"
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"
)

func main() {
	setUpLogger()
	ctx, cancel := context.WithTimeout(context.Background(), time.Hour)
	scanner := bufio.NewScanner(os.Stdin)
	go readInput(cancel, scanner)
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
	fmu := caching.NewTable()
	caching.InitPaths(".")
	cacheManager := caching.InitCache(ctx, fmu)

	handler := request.NewHandler(ctx, ldb, cacheManager, fmu)
	request.StartReqHandling(handler)
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

func readInput(cancel context.CancelFunc, scanner *bufio.Scanner) {
	for scanner.Scan() {
		line := scanner.Text()
		if line == "exit" {
			cancel()
			break
		}
	}
}
