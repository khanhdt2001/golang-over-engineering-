package main

import (
	"log"
	"log/slog"
	"net/http"
	"os"

	"over-engineering/user/api"
	"over-engineering/user/db"
	"over-engineering/user/service"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://app:app@localhost:15432/users?sslmode=disable"
	}
	repository, err := db.Open(dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer repository.Close()
	if err := repository.Migrate(); err != nil {
		log.Fatal(err)
	}

	log.Println("user API listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", api.New(service.New(repository))))
}
