package db

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Conversation represents a chat session.
type Conversation struct {
	ID           string    `json:"id"`
	UserID       int64     `json:"user_id"`
	Title        string    `json:"title"`
	Provider     string    `json:"provider"`
	Model        string    `json:"model"`
	SystemPrompt string    `json:"system_prompt"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Message represents a single chat message.
type Message struct {
	ID             int64     `json:"id"`
	ConversationID string    `json:"conversation_id"`
	Role           string    `json:"role"`
	Content        string    `json:"content"`
	TokenCount     int       `json:"token_count"`
	CreatedAt      time.Time `json:"created_at"`
}

// CreateConversation inserts a new conversation and returns it.
func CreateConversation(db *sql.DB, userID int64, provider, model, systemPrompt string) (*Conversation, error) {
	id := uuid.New().String()
	_, err := db.Exec(
		`INSERT INTO conversations (id, user_id, provider, model, system_prompt) VALUES (?, ?, ?, ?, ?)`,
		id, userID, provider, model, systemPrompt,
	)
	if err != nil {
		return nil, err
	}
	return GetConversation(db, id, userID, "admin") // bypass check since it's just created
}

// GetConversation retrieves a conversation by ID.
func GetConversation(db *sql.DB, id string, currentUserID int64, role string) (*Conversation, error) {
	var row *sql.Row
	if role == "admin" {
		row = db.QueryRow(`SELECT id, user_id, title, provider, model, COALESCE(system_prompt,''), created_at, updated_at FROM conversations WHERE id = ?`, id)
	} else {
		row = db.QueryRow(`SELECT id, user_id, title, provider, model, COALESCE(system_prompt,''), created_at, updated_at FROM conversations WHERE id = ? AND user_id = ?`, id, currentUserID)
	}
	c := &Conversation{}
	var uid sql.NullInt64
	err := row.Scan(&c.ID, &uid, &c.Title, &c.Provider, &c.Model, &c.SystemPrompt, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if uid.Valid {
		c.UserID = uid.Int64
	}
	return c, nil
}

// ListConversations returns all conversations ordered by updated_at desc.
func ListConversations(db *sql.DB, currentUserID int64, role string) ([]Conversation, error) {
	var rows *sql.Rows
	var err error
	if role == "admin" {
		rows, err = db.Query(`SELECT id, user_id, title, provider, model, COALESCE(system_prompt,''), created_at, updated_at FROM conversations ORDER BY updated_at DESC`)
	} else {
		rows, err = db.Query(`SELECT id, user_id, title, provider, model, COALESCE(system_prompt,''), created_at, updated_at FROM conversations WHERE user_id = ? ORDER BY updated_at DESC`, currentUserID)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Conversation
	for rows.Next() {
		var c Conversation
		var uid sql.NullInt64
		if err := rows.Scan(&c.ID, &uid, &c.Title, &c.Provider, &c.Model, &c.SystemPrompt, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		if uid.Valid {
			c.UserID = uid.Int64
		}
		out = append(out, c)
	}
	return out, nil
}

// UpdateConversation updates title and system_prompt.
func UpdateConversation(db *sql.DB, id, title, systemPrompt string, currentUserID int64, role string) error {
	if role != "admin" {
		var uid sql.NullInt64
		err := db.QueryRow(`SELECT user_id FROM conversations WHERE id=?`, id).Scan(&uid)
		if err != nil {
			return err
		}
		if !uid.Valid || uid.Int64 != currentUserID {
			return fmt.Errorf("unauthorized to update this conversation")
		}
	}
	_, err := db.Exec(`UPDATE conversations SET title=?, system_prompt=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`, title, systemPrompt, id)
	return err
}

// DeleteConversation deletes a conversation (messages cascade).
func DeleteConversation(db *sql.DB, id string, currentUserID int64, role string) error {
	if role != "admin" {
		var uid sql.NullInt64
		err := db.QueryRow(`SELECT user_id FROM conversations WHERE id=?`, id).Scan(&uid)
		if err != nil {
			return err
		}
		if !uid.Valid || uid.Int64 != currentUserID {
			return fmt.Errorf("unauthorized to delete this conversation")
		}
	}
	_, err := db.Exec(`DELETE FROM conversations WHERE id=?`, id)
	return err
}

// SaveMessage appends a message to a conversation.
func SaveMessage(db *sql.DB, convID, role, content string, tokenCount int, currentUserID int64, userRole string) (*Message, error) {
	if userRole != "admin" {
		var uid sql.NullInt64
		err := db.QueryRow(`SELECT user_id FROM conversations WHERE id=?`, convID).Scan(&uid)
		if err != nil {
			return nil, err
		}
		if !uid.Valid || uid.Int64 != currentUserID {
			return nil, fmt.Errorf("unauthorized to add messages to this conversation")
		}
	}

	res, err := db.Exec(`INSERT INTO messages (conversation_id, role, content, token_count) VALUES (?, ?, ?, ?)`, convID, role, content, tokenCount)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	// touch updated_at on the parent conversation
	_, _ = db.Exec(`UPDATE conversations SET updated_at=CURRENT_TIMESTAMP WHERE id=?`, convID)
	return &Message{ID: id, ConversationID: convID, Role: role, Content: content, TokenCount: tokenCount}, nil
}

// GetMessages returns all messages for a conversation ordered by id.
func GetMessages(db *sql.DB, convID string, currentUserID int64, role string) ([]Message, error) {
	if role != "admin" {
		var uid sql.NullInt64
		err := db.QueryRow(`SELECT user_id FROM conversations WHERE id=?`, convID).Scan(&uid)
		if err != nil {
			return nil, err
		}
		if !uid.Valid || uid.Int64 != currentUserID {
			return nil, fmt.Errorf("unauthorized to access this conversation")
		}
	}

	rows, err := db.Query(`SELECT id, conversation_id, role, content, COALESCE(token_count,0), created_at FROM messages WHERE conversation_id=? ORDER BY id ASC`, convID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Message
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.ConversationID, &m.Role, &m.Content, &m.TokenCount, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, nil
}

// DeleteMessage deletes a single message by ID.
func DeleteMessage(db *sql.DB, id int64, currentUserID int64, role string) error {
	if role != "admin" {
		var uid sql.NullInt64
		err := db.QueryRow(`
			SELECT c.user_id 
			FROM messages m 
			JOIN conversations c ON m.conversation_id = c.id 
			WHERE m.id = ?`, id).Scan(&uid)
		if err != nil {
			return err
		}
		if !uid.Valid || uid.Int64 != currentUserID {
			return fmt.Errorf("unauthorized to delete this message")
		}
	}
	_, err := db.Exec(`DELETE FROM messages WHERE id=?`, id)
	return err
}
