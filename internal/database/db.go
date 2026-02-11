package database

import (
	"database/sql"
	"embed"
	"fmt"
	"log" // Added log for migration messages
	"os"
	"path/filepath"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/sqlite" // Pure Go sqlite driver
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

//go:generate sh -c "cd ../.. && sqlc generate"

// DB provides a centralized database connection
type DB struct {
	SQL *sql.DB
}

// NewDB initializes the SQLite database and runs migrations.
func NewDB(dbPath string) (*DB, error) {
	// Ensure directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	// Run migrations before opening the database connection for the app
	// This ensures the schema is always up-to-date
	if err := RunMigrations(dbPath); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	return &DB{SQL: db}, nil
}

// Close closes the database connection.
func (d *DB) Close() error {
	return d.SQL.Close()
}

// RunMigrations applies database migrations using golang-migrate.
func RunMigrations(databasePath string) error {
	d, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("failed to create iofs driver: %w", err)
	}

	// Migrate expects a URL-like string for the database source
	// For modernc.org/sqlite, it's "sqlite://<path_to_db>"
	databaseURL := fmt.Sprintf("sqlite://%s", databasePath)

	m, err := migrate.NewWithSourceInstance(
		"iofs",
		d,
		databaseURL,
	)
	if err != nil {
		return fmt.Errorf("failed to create migrate instance: %w", err)
	}

	// Apply all available migrations
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to apply migrations: %w", err)
	}

	log.Println("Database migrations applied successfully!")
	return nil
}
