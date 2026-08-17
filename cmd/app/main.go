package main

import (
	"log/slog"
	"os"
)

func main() {
	//setUpLogger()
	//ctx, cancel := context.WithTimeout(context.Background(), time.Hour)
	//ldb, err := db.Connect()
	//defer ldb.Close()
	//fmu := caching.NewTable()
	//caching.InitPaths(".")
	//cacheManager := caching.InitCache(ctx, fmu)

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
