package main

import (
	"errors"
	"flag"
	"fmt"
	"log"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func main() {
	dir := flag.String("dir", "file://./migrations", "directory with migrations")
	dsn := flag.String("dsn", "", "database connection string")
	action := flag.String("action", "up", "migration action: up, down")

	flag.Parse()

	if *dsn == "" {
		log.Fatal("dsn is required")
	}

	m, err := migrate.New(*dir, *dsn)
	if err != nil {
		log.Fatalf("failed to create migrate instance: %v", err)
	}

	switch *action {
	case "up":
		err = m.Up()
	case "down":
		err = m.Down()
	default:
		log.Fatalf("unknown action: %s", *action)
	}

	if err != nil && !errors.Is(err, migrate.ErrNoChange) {
		log.Fatalf("migration failed: %v", err)
	}

	fmt.Println("migration done successfully")
}
