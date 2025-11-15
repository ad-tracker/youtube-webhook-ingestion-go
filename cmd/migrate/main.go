package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func main() {
	var (
		dbURL          string
		migrationsPath string
		direction      string
		steps          int
	)

	flag.StringVar(&dbURL, "db", "", "Database URL (e.g., postgres://user:pass@localhost:5432/dbname?sslmode=disable)")
	flag.StringVar(&migrationsPath, "path", "./migrations", "Path to migrations directory")
	flag.StringVar(&direction, "direction", "up", "Migration direction: up, down, or version")
	flag.IntVar(&steps, "steps", 0, "Number of steps to migrate (0 means all)")
	flag.Parse()

	if dbURL == "" {
		dbURL = os.Getenv("DATABASE_URL")
	}

	if dbURL == "" {
		log.Fatal("Database URL must be provided via -db flag or DATABASE_URL environment variable")
	}

	m, err := migrate.New(
		fmt.Sprintf("file://%s", migrationsPath),
		dbURL,
	)
	if err != nil {
		log.Fatalf("Failed to create migrate instance: %v", err)
	}
	defer m.Close()

	switch direction {
	case "up":
		if steps > 0 {
			err = m.Steps(steps)
		} else {
			err = m.Up()
		}
	case "down":
		if steps > 0 {
			err = m.Steps(-steps)
		} else {
			err = m.Down()
		}
	default:
		log.Fatalf("Invalid direction: %s (must be 'up' or 'down')", direction)
	}

	if err != nil && err != migrate.ErrNoChange {
		log.Fatalf("Migration failed: %v", err)
	}

	version, dirty, err := m.Version()
	if err != nil && err != migrate.ErrNilVersion {
		log.Fatalf("Failed to get migration version: %v", err)
	}

	if err == migrate.ErrNilVersion {
		log.Println("Migration completed successfully (no version)")
	} else {
		log.Printf("Migration completed successfully (version: %d, dirty: %t)", version, dirty)
	}
}
