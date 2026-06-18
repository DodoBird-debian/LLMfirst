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
	return db, nil
}

// Migrate runs the embedded schema SQL against the database.
func Migrate(db *sql.DB) error {
	_, err := db.Exec(schema)
	return err
}
