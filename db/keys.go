package db

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"time"
)

// deriveKey creates a 32-byte key from the secret string.
func deriveKey(secret string) []byte {
	hash := sha256.Sum256([]byte(secret))
	return hash[:]
}

// encrypt encrypts text using AES-GCM if secret is provided.
func encrypt(secret, text string) (string, error) {
	if secret == "" || text == "" {
		return text, nil
	}
	block, err := aes.NewCipher(deriveKey(secret))
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := gcm.Seal(nonce, nonce, []byte(text), nil)
	return hex.EncodeToString(ciphertext), nil
}

// decrypt decrypts text using AES-GCM if secret is provided.
func decrypt(secret, cryptoText string) (string, error) {
	if secret == "" || cryptoText == "" {
		return cryptoText, nil
	}
	data, err := hex.DecodeString(cryptoText)
	if err != nil {
		return cryptoText, nil // Fallback to plain if it was unencrypted
	}
	block, err := aes.NewCipher(deriveKey(secret))
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(data) < gcm.NonceSize() {
		return cryptoText, nil // Fallback
	}
	nonce, ciphertext := data[:gcm.NonceSize()], data[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return cryptoText, nil // Fallback to plain text on decryption failure
	}
	return string(plaintext), nil
}

// APIKey represents a stored provider API key.
type APIKey struct {
	ID        int64     `json:"id"`
	UserID    *int64    `json:"user_id"`    // NULL for shared keys
	IsShared  bool      `json:"is_shared"`  // True if key is global/shared
	Provider  string    `json:"provider"`
	Label     string    `json:"label"`
	KeyValue  string    `json:"key_value,omitempty"` // redacted when listing
	BaseURL   string    `json:"base_url"`
	CreatedAt time.Time `json:"created_at"`
}

// CreateKey inserts a new API key.
func CreateKey(db *sql.DB, secret string, userID *int64, isShared bool, provider, label, keyValue, baseURL string) (*APIKey, error) {
	encVal, err := encrypt(secret, keyValue)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt key: %v", err)
	}
	sharedVal := 0
	if isShared {
		sharedVal = 1
	}
	res, err := db.Exec(`INSERT INTO api_keys (user_id, is_shared, provider, label, key_value, base_url) VALUES (?, ?, ?, ?, ?, ?)`, userID, sharedVal, provider, label, encVal, baseURL)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return &APIKey{ID: id, UserID: userID, IsShared: isShared, Provider: provider, Label: label, BaseURL: baseURL}, nil
}

// ListKeys returns keys visible to the user. Key_value is redacted to last 4 chars.
func ListKeys(db *sql.DB, secret string, currentUserID int64, role string) ([]APIKey, error) {
	var rows *sql.Rows
	var err error
	if role == "admin" {
		rows, err = db.Query(`SELECT id, user_id, is_shared, provider, label, key_value, COALESCE(base_url,''), created_at FROM api_keys ORDER BY id`)
	} else {
		rows, err = db.Query(`SELECT id, user_id, is_shared, provider, label, key_value, COALESCE(base_url,''), created_at FROM api_keys WHERE user_id = ? OR is_shared = 1 ORDER BY id`, currentUserID)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []APIKey
	for rows.Next() {
		var k APIKey
		var kv string
		var uid sql.NullInt64
		var sharedVal int
		if err := rows.Scan(&k.ID, &uid, &sharedVal, &k.Provider, &k.Label, &kv, &k.BaseURL, &k.CreatedAt); err != nil {
			return nil, err
		}
		if uid.Valid {
			val := uid.Int64
			k.UserID = &val
		}
		k.IsShared = (sharedVal == 1)
		// decrypt before redacting
		plainKV, _ := decrypt(secret, kv)
		// redact: show only last 4 chars
		if len(plainKV) > 4 {
			k.KeyValue = "****" + plainKV[len(plainKV)-4:]
		} else {
			k.KeyValue = "****"
		}
		out = append(out, k)
	}
	return out, nil
}

// GetKeyValue returns the raw (unredacted) key_value for a key ID if authorized.
func GetKeyValue(db *sql.DB, secret string, id int64, currentUserID int64, role string) (string, string, error) {
	var kv, baseURL string
	var uid sql.NullInt64
	var sharedVal int
	err := db.QueryRow(`SELECT key_value, COALESCE(base_url,''), user_id, is_shared FROM api_keys WHERE id=?`, id).Scan(&kv, &baseURL, &uid, &sharedVal)
	if err != nil {
		return "", "", err
	}

	// Authorization check
	if role != "admin" {
		isOwner := uid.Valid && uid.Int64 == currentUserID
		isShared := sharedVal == 1
		if !isOwner && !isShared {
			return "", "", fmt.Errorf("unauthorized to use this API key")
		}
	}

	plainKV, err := decrypt(secret, kv)
	return plainKV, baseURL, err
}

// UpdateKey updates label and base_url for a key if authorized.
func UpdateKey(db *sql.DB, id int64, label, baseURL string, currentUserID int64, role string) error {
	if role != "admin" {
		var uid sql.NullInt64
		err := db.QueryRow(`SELECT user_id FROM api_keys WHERE id=?`, id).Scan(&uid)
		if err != nil {
			return err
		}
		if !uid.Valid || uid.Int64 != currentUserID {
			return fmt.Errorf("unauthorized to update this API key")
		}
	}
	_, err := db.Exec(`UPDATE api_keys SET label=?, base_url=? WHERE id=?`, label, baseURL, id)
	return err
}

// DeleteKey removes an API key if authorized.
func DeleteKey(db *sql.DB, id int64, currentUserID int64, role string) error {
	if role != "admin" {
		var uid sql.NullInt64
		err := db.QueryRow(`SELECT user_id FROM api_keys WHERE id=?`, id).Scan(&uid)
		if err != nil {
			return err
		}
		if !uid.Valid || uid.Int64 != currentUserID {
			return fmt.Errorf("unauthorized to delete this API key")
		}
	}
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
