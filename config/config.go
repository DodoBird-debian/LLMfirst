package config

import (
    "flag"
)

type Config struct {
    Host       string
    Port       string
    DBPath     string
    OllamaURL  string
    Secret     string
}

func ParseFlags() Config {
    host := flag.String("host", "0.0.0.0", "Host address to bind")
    port := flag.String("port", "42068", "Port number to listen on")
    db := flag.String("db", "data.db", "Path to SQLite database file")
    ollama := flag.String("ollama-url", "http://localhost:11434", "Base URL for Ollama provider")
    secret := flag.String("secret", "", "Passphrase for optional AES encryption of API keys")
    flag.Parse()
    return Config{Host: *host, Port: *port, DBPath: *db, OllamaURL: *ollama, Secret: *secret}
}
