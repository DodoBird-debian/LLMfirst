package db

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
)

// Conversation represents a chat session.
type Conversation struct {
	ID           string    `json:"id"`
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
func CreateConversation(db *sql.DB, provider, model, systemPrompt string) (*Conversation, error) {
	id := uuid.New().String()
	_, err := db.Exec(
		`INSERT INTO conversations (id, provider, model, system_prompt) VALUES (?, ?, ?, ?)`,
		id, provider, model, systemPrompt,
	)
	if err != nil {
		return nil, err
	}
	return GetConversation(db, id)
}

// GetConversation retrieves a conversation by ID.
func GetConversation(db *sql.DB, id string) (*Conversation, error) {
	row := db.QueryRow(`SELECT id, title, provider, model, COALESCE(system_prompt,''), created_at, updated_at FROM conversations WHERE id = ?`, id)
	c := &Conversation{}
	err := row.Scan(&c.ID, &c.Title, &c.Provider, &c.Model, &c.SystemPrompt, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return c, nil
}

// ListConversations returns all conversations ordered by updated_at desc.
func ListConversations(db *sql.DB) ([]Conversation, error) {
	rows, err := db.Query(`SELECT id, title, provider, model, COALESCE(system_prompt,''), created_at, updated_at FROM conversations ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Conversation
	for rows.Next() {
		var c Conversation
		if err := rows.Scan(&c.ID, &c.Title, &c.Provider, &c.Model, &c.SystemPrompt, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, nil
}

// UpdateConversation updates title and system_prompt.
func UpdateConversation(db *sql.DB, id, title, systemPrompt string) error {
	_, err := db.Exec(`UPDATE conversations SET title=?, system_prompt=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`, title, systemPrompt, id)
	return err
}

// DeleteConversation deletes a conversation (messages cascade).
func DeleteConversation(db *sql.DB, id string) error {
	_, err := db.Exec(`DELETE FROM conversations WHERE id=?`, id)
	return err
}

// SaveMessage appends a message to a conversation.
func SaveMessage(db *sql.DB, convID, role, content string, tokenCount int) (*Message, error) {
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
func GetMessages(db *sql.DB, convID string) ([]Message, error) {
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
func DeleteMessage(db *sql.DB, id int64) error {
	_, err := db.Exec(`DELETE FROM messages WHERE id=?`, id)
	return err
}
