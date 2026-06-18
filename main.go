package main

import (
	"log"
	"net/http"

	"github.com/dodobird/llm-webui/config"
	"github.com/dodobird/llm-webui/db"
	"github.com/dodobird/llm-webui/server"
)

func main() {
	cfg := config.ParseFlags()

	// Open SQLite DB (creates if missing)
	sqlDB, err := db.OpenDB(cfg.DBPath)
	if err != nil {
		log.Fatalf("failed to open DB: %v", err)
	}
	defer sqlDB.Close()

	// Run migrations (schema.sql embedded)
	if err := db.Migrate(sqlDB); err != nil {
		log.Fatalf("migration error: %v", err)
	}
	log.Printf("Database: %s", cfg.DBPath)

	// Initialize server with router
	r := server.NewRouter(sqlDB, cfg)
	addr := cfg.Host + ":" + cfg.Port
	log.Printf("LLM WebUI running at http://localhost:%s", cfg.Port)
	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
