package main

import (
	"log"
	"log/slog"
	"net/http"
	"os"

	"over-engineering/task/api"
	"over-engineering/task/db"
	"over-engineering/task/service"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://app:app@localhost:25432/tasks?sslmode=disable"
	}
	repository, err := db.Open(dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer repository.Close()
	if err := repository.Migrate(); err != nil {
		log.Fatal(err)
	}

	log.Println("task API listening on :8081")
	log.Fatal(http.ListenAndServe(":8081", api.New(service.New(repository))))
}
