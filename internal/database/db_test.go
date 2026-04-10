package database

import (
	"path/filepath"
	"testing"
)

func TestMigrateUp(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := NewDB(dbPath)
	if err != nil {
		t.Fatalf("Failed to create DB: %v", err)
	}
	defer db.Close()

	if err := db.MigrateUp(dbPath); err != nil {
		t.Fatalf("MigrateUp failed: %v", err)
	}

	// Verify 'status' column in 'user_meal_plans'
	var columnName string
	err = db.SQL.QueryRow("SELECT name FROM pragma_table_info('user_meal_plans') WHERE name='status'").Scan(&columnName)
	if err != nil {
		t.Errorf("Failed to find 'status' column in 'user_meal_plans': %v", err)
	}

	// Verify 'audit_logs' table exists (from migration 007)
	var tableName string
	err = db.SQL.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='audit_logs'").Scan(&tableName)
	if err != nil {
		t.Errorf("Failed to find 'audit_logs' table: %v", err)
	}
}
