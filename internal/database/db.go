package database

import (
	_ "embed"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite" // SQLite driver
)

//go:embed schema.sql
var schemaSQL string

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

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	d := &DB{SQL: db}
	if err := d.migrate(); err != nil {
		return nil, err
	}

	return d, nil
}

// Close closes the database connection.
func (d *DB) Close() error {
	return d.SQL.Close()
}

// migrate runs all necessary schema migrations.
func (d *DB) migrate() error {
	_, err := d.SQL.Exec(schemaSQL)
	if err != nil {
		return fmt.Errorf("failed to run schema migrations: %w", err)
	}
	return nil
}
