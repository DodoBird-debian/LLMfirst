package server

import (
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/dodobird/llm-webui/db"
	"github.com/dodobird/llm-webui/pkg/fileprocessor"
)

// handleFileUpload handles POST /api/upload
func handleFileUpload(database *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Parse multipart form (max 50 MB)
		if err := r.ParseMultipartForm(50 << 20); err != nil {
			http.Error(w, "File too large or invalid form data", http.StatusBadRequest)
			return
		}

		convID := r.FormValue("conversationId")
		if convID == "" {
			http.Error(w, "conversationId is required", http.StatusBadRequest)
			return
		}

		file, header, err := r.FormFile("file")
		if err != nil {
			http.Error(w, "Failed to read file", http.StatusBadRequest)
			return
		}
		defer file.Close()

		// Create data/uploads directory if not exists
		uploadDir := filepath.Join(".", "data", "uploads")
		if err := os.MkdirAll(uploadDir, 0755); err != nil {
			http.Error(w, "Server error creating upload directory", http.StatusInternalServerError)
			return
		}

		fileID := uuid.New().String()
		ext := filepath.Ext(header.Filename)
		filename := fileID + ext
		storagePath := filepath.Join(uploadDir, filename)

		// Save file to disk
		out, err := os.Create(storagePath)
		if err != nil {
			http.Error(w, "Server error saving file", http.StatusInternalServerError)
			return
		}
		defer out.Close()

		if _, err := io.Copy(out, file); err != nil {
			http.Error(w, "Server error writing file", http.StatusInternalServerError)
			return
		}

		// Process file
		res, err := fileprocessor.ProcessFile(storagePath, header.Header.Get("Content-Type"), header.Filename)
		extractedText := ""
		imageBase64 := ""
		if err == nil {
			extractedText = res.ExtractedText
			imageBase64 = res.ImageBase64
		}

		// Save to DB (We'll update SaveAttachment to accept imageBase64)
		if err := db.SaveAttachment(database, fileID, convID, header.Filename, header.Header.Get("Content-Type"), storagePath, extractedText, imageBase64); err != nil {
			http.Error(w, "Server error saving to database", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{
			"id":       fileID,
			"filename": header.Filename,
		})
	}
}

// handleDeleteAttachment handles DELETE /api/attachments/{id}
func handleDeleteAttachment(database *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		
		// Optional: delete file from disk here if you want to clean up

		if err := db.DeleteAttachment(database, id); err != nil {
			http.Error(w, "Failed to delete attachment", http.StatusInternalServerError)
			return
		}
		
		w.WriteHeader(http.StatusOK)
	}
}
