package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/joho/godotenv"
	"github.com/opencron/opencron/internal/engine"
	"github.com/opencron/opencron/internal/handlers"
	"github.com/opencron/opencron/internal/store"
)

func main() {
	_ = godotenv.Load()

	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = "."
	}

	// Ensure data directory exists
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		log.Fatalf("Failed to create data directory: %v", err)
	}

	dbPath := filepath.Join(dataDir, "opencron.db")
	s, err := store.New(dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize store: %v", err)
	}

	retentionHours := 48
	if val := os.Getenv("LOG_RETENTION_HOURS"); val != "" {
		if h, err := strconv.Atoi(val); err == nil {
			retentionHours = h
		}
	}
	retention := time.Duration(retentionHours) * time.Hour

	e := engine.New(s, dataDir, retention)
	e.Start()

	api := &handlers.API{
		Store:   s,
		Engine:  e,
		DataDir: dataDir,
	}

	http.HandleFunc("/", api.ServeHTTP)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Opencron starting on :%s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
