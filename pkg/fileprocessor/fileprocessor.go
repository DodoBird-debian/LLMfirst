package fileprocessor

import (
	"encoding/base64"
	"io"
	"os"
	"strings"
)

// Result holds the processed data from a file
type Result struct {
	ExtractedText string
	ImageBase64   string
}

// ProcessFile analyzes a file based on its extension/mime type and returns processed data.
// This is a modular component that can be expanded to 10k LOC for complex PDF OCR, etc.
func ProcessFile(filepath, mimeType, filename string) (*Result, error) {
	res := &Result{}

	ext := strings.ToLower(filename)
	isText := false
	if strings.HasSuffix(ext, ".txt") || strings.HasSuffix(ext, ".md") || strings.HasSuffix(ext, ".csv") ||
		strings.HasSuffix(ext, ".json") || strings.HasSuffix(ext, ".log") || strings.HasSuffix(ext, ".go") ||
		strings.HasSuffix(ext, ".js") || strings.HasSuffix(ext, ".html") || strings.HasSuffix(ext, ".css") {
		isText = true
	}

	isImage := false
	if strings.HasPrefix(mimeType, "image/") || strings.HasSuffix(ext, ".png") || strings.HasSuffix(ext, ".jpg") ||
		strings.HasSuffix(ext, ".jpeg") || strings.HasSuffix(ext, ".webp") || strings.HasSuffix(ext, ".gif") {
		isImage = true
	}

	if isText {
		bytes, err := os.ReadFile(filepath)
		if err == nil {
			res.ExtractedText = string(bytes)
		}
	} else if isImage {
		f, err := os.Open(filepath)
		if err == nil {
			defer f.Close()
			bytes, err := io.ReadAll(f)
			if err == nil {
				b64 := base64.StdEncoding.EncodeToString(bytes)
				res.ImageBase64 = "data:" + mimeType + ";base64," + b64
			}
		}
	}

	return res, nil
}
