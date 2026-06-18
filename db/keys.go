package db

import (
	"database/sql"
	"time"
)

// APIKey represents a stored provider API key.
type APIKey struct {
	ID        int64     `json:"id"`
	Provider  string    `json:"provider"`
	Label     string    `json:"label"`
	KeyValue  string    `json:"key_value,omitempty"` // redacted when listing
	BaseURL   string    `json:"base_url"`
	CreatedAt time.Time `json:"created_at"`
}

// CreateKey inserts a new API key.
func CreateKey(db *sql.DB, provider, label, keyValue, baseURL string) (*APIKey, error) {
	res, err := db.Exec(`INSERT INTO api_keys (provider, label, key_value, base_url) VALUES (?, ?, ?, ?)`, provider, label, keyValue, baseURL)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return &APIKey{ID: id, Provider: provider, Label: label, BaseURL: baseURL}, nil
}

// ListKeys returns all keys with key_value redacted to last 4 chars.
func ListKeys(db *sql.DB) ([]APIKey, error) {
	rows, err := db.Query(`SELECT id, provider, label, key_value, COALESCE(base_url,''), created_at FROM api_keys ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []APIKey
	for rows.Next() {
		var k APIKey
		var kv string
		if err := rows.Scan(&k.ID, &k.Provider, &k.Label, &kv, &k.BaseURL, &k.CreatedAt); err != nil {
			return nil, err
		}
		// redact: show only last 4 chars
		if len(kv) > 4 {
			k.KeyValue = "****" + kv[len(kv)-4:]
		} else {
			k.KeyValue = "****"
		}
		out = append(out, k)
	}
	return out, nil
}

// GetKeyValue returns the raw (unredacted) key_value for a key ID.
func GetKeyValue(db *sql.DB, id int64) (string, string, error) {
	var kv, baseURL string
	err := db.QueryRow(`SELECT key_value, COALESCE(base_url,'') FROM api_keys WHERE id=?`, id).Scan(&kv, &baseURL)
	return kv, baseURL, err
}

// UpdateKey updates label and base_url for a key.
func UpdateKey(db *sql.DB, id int64, label, baseURL string) error {
	_, err := db.Exec(`UPDATE api_keys SET label=?, base_url=? WHERE id=?`, label, baseURL, id)
	return err
}

// DeleteKey removes an API key.
func DeleteKey(db *sql.DB, id int64) error {
	_, err := db.Exec(`DELETE FROM api_keys WHERE id=?`, id)
	return err
}

// GetSetting retrieves a setting value by key.
func GetSetting(db *sql.DB, key string) (string, error) {
	var val string
	err := db.QueryRow(`SELECT value FROM settings WHERE key=?`, key).Scan(&val)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return val, err
}

// SetSetting upserts a setting key/value.
func SetSetting(db *sql.DB, key, value string) error {
	_, err := db.Exec(`INSERT INTO settings (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`, key, value)
	return err
}
