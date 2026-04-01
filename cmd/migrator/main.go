package main

import (
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"log"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/markosoft2000/auth/internal/config"
)

func main() {
	var migrationsPath string
	flag.StringVar(&migrationsPath, "migrations-path", "migrations", "path to migrations folder")
	flag.Parse()

	// Validate command argument (up/down)
	if len(flag.Args()) == 0 {
		log.Fatal("error: must specify a command (up or down)")
	}
	cmd := flag.Arg(0)

	dbcfg := config.MustLoad().Postgres

	log.Printf("Starting database migration process (path: %s)", migrationsPath)

	connectionString := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
		dbcfg.User,
		dbcfg.Password,
		dbcfg.Host,
		dbcfg.Port,
		dbcfg.Database,
		dbcfg.SSLMode,
	)

	db, err := sql.Open("postgres", connectionString)
	if err != nil {
		log.Fatalf("Unable to connect to database: %v", err)
	}
	defer db.Close()

	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		log.Fatal(err)
	}
	defer driver.Close()

	m, err := migrate.NewWithDatabaseInstance("file://migrations", "postgres", driver)
	if err != nil {
		log.Fatal(err)
	}

	switch cmd {
	case "up":
		if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			log.Fatalf("Migration up failed: %v", err)
		}
		log.Println("Migration up completed successfully")
	case "down":
		if err := m.Down(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			log.Fatalf("Migration down failed: %v", err)
		}
		log.Println("Migration down completed successfully")
	}

	log.Println("Migrations applied successfully!")
}
