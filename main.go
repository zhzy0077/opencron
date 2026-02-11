package main

import (
	"log"
	"net/http"
	"os"

	"github.com/opencron/opencron/internal/engine"
	"github.com/opencron/opencron/internal/handlers"
	"github.com/opencron/opencron/internal/store"
)

func main() {
	dbPath := "opencron.db"
	s, err := store.New(dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize store: %v", err)
	}

	e := engine.New(s)
	e.Start()

	api := &handlers.API{
		Store:  s,
		Engine: e,
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
