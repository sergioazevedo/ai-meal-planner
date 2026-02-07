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
	// 1. Ensure user_meal_plans has the latest columns if it already exists.
	// We check for column existence before trying to add them.
	columnCheck := func(table, column string) bool {
		var name string
		query := fmt.Sprintf("SELECT name FROM pragma_table_info('%s') WHERE name='%s'", table, column)
		err := d.SQL.QueryRow(query).Scan(&name)
		return err == nil
	}

	// Evolution: Add week_start_date if missing
	if !columnCheck("user_meal_plans", "week_start_date") {
		// Attempt to add it. This will fail if the table doesn't exist yet, which is fine
		// because the base schema will create it with the column.
		_, _ = d.SQL.Exec("ALTER TABLE user_meal_plans ADD COLUMN week_start_date DATETIME NOT NULL DEFAULT '1970-01-01 00:00:00';")
	}

	// Evolution: Add created_at if missing
	if !columnCheck("user_meal_plans", "created_at") {
		_, _ = d.SQL.Exec("ALTER TABLE user_meal_plans ADD COLUMN created_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL;")
	}

	// 2. Run the base schema (CREATE TABLE IF NOT EXISTS, CREATE INDEX IF NOT EXISTS)
	_, err := d.SQL.Exec(schemaSQL)
	if err != nil {
		return fmt.Errorf("failed to run schema migrations: %w", err)
	}

	return nil
}
