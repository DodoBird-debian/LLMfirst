package db

import (
	"database/sql"
	_ "embed"
	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schema string

// OpenDB opens (or creates) the SQLite database at the given path.
func OpenDB(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		return nil, err
	}
	// Enable explicit foreign key constraints and standard DELETE journaling mode (no extra files)
	if _, err := db.Exec(`PRAGMA foreign_keys = ON;`); err != nil {
		return nil, err
	}
	if _, err := db.Exec(`PRAGMA journal_mode = DELETE;`); err != nil {
		return nil, err
	}
	return db, nil
}

// Migrate runs the embedded schema SQL against the database and executes migrations if columns are missing.
func Migrate(db *sql.DB) error {
	// 1. Run schema.sql (creates new tables/columns for fresh databases)
	if _, err := db.Exec(schema); err != nil {
		return err
	}

	// 2. Perform migrations for existing databases
	// Add user_id to api_keys if missing
	if _, err := db.Exec("SELECT user_id FROM api_keys LIMIT 1"); err != nil {
		if _, alterErr := db.Exec("ALTER TABLE api_keys ADD COLUMN user_id INTEGER REFERENCES users(id) ON DELETE CASCADE;"); alterErr != nil {
			return alterErr
		}
	}

	// Add is_shared to api_keys if missing
	if _, err := db.Exec("SELECT is_shared FROM api_keys LIMIT 1"); err != nil {
		if _, alterErr := db.Exec("ALTER TABLE api_keys ADD COLUMN is_shared INTEGER DEFAULT 0;"); alterErr != nil {
			return alterErr
		}
	}

	// Add user_id to conversations if missing
	if _, err := db.Exec("SELECT user_id FROM conversations LIMIT 1"); err != nil {
		if _, alterErr := db.Exec("ALTER TABLE conversations ADD COLUMN user_id INTEGER REFERENCES users(id) ON DELETE CASCADE;"); alterErr != nil {
			return alterErr
		}
	}

	return nil
}
