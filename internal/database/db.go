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

// NewDB initializes the SQLite database connection **without** running migrations.
func NewDB(dbPath string) (*DB, error) {
	// Ensure directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
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

// MigrateUp applies all available database migrations.
func (d *DB) MigrateUp(databasePath string) error {
	migrations, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("failed to create migration source: %w", err)
	}

	// The database URL for modernc.org/sqlite is "sqlite://<path>"
	databaseURL := fmt.Sprintf("sqlite://%s", databasePath)

	m, err := migrate.NewWithSourceInstance("iofs", migrations, databaseURL)
	if err != nil {
		return fmt.Errorf("failed to create migrate instance: %w", err)
	}

	// Apply all available "up" migrations
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to apply migrations: %w", err)
	}

	log.Println("Database migrations applied successfully!")
	return nil
}
