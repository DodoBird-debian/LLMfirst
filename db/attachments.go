package db

import (
	"database/sql"
	"time"
)

type Attachment struct {
	ID             string    `json:"id"`
	ConversationID string    `json:"conversation_id"`
	Filename       string    `json:"filename"`
	FileType       string    `json:"file_type"`
	StoragePath    string    `json:"storage_path"`
	ExtractedText  string    `json:"extracted_text"`
	ImageBase64    string    `json:"image_base64"`
	CreatedAt      time.Time `json:"created_at"`
}

func SaveAttachment(db *sql.DB, id, conversationID, filename, fileType, storagePath, extractedText, imageBase64 string) error {
	query := `
		INSERT INTO attachments (id, conversation_id, filename, file_type, storage_path, extracted_text, image_base64)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`
	_, err := db.Exec(query, id, conversationID, filename, fileType, storagePath, extractedText, imageBase64)
	return err
}

func GetAttachmentsByConversation(db *sql.DB, conversationID string) ([]Attachment, error) {
	query := `
		SELECT id, conversation_id, filename, file_type, storage_path, extracted_text, image_base64, created_at
		FROM attachments
		WHERE conversation_id = ?
		ORDER BY created_at ASC
	`
	rows, err := db.Query(query, conversationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var attachments []Attachment
	for rows.Next() {
		var a Attachment
		if err := rows.Scan(&a.ID, &a.ConversationID, &a.Filename, &a.FileType, &a.StoragePath, &a.ExtractedText, &a.ImageBase64, &a.CreatedAt); err != nil {
			return nil, err
		}
		attachments = append(attachments, a)
	}
	return attachments, nil
}

func DeleteAttachment(db *sql.DB, id string) error {
	_, err := db.Exec("DELETE FROM attachments WHERE id = ?", id)
	return err
}
