package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/calvinwijaya/card-games-be/internal/api"
	"github.com/calvinwijaya/card-games-be/internal/db"
	"github.com/calvinwijaya/card-games-be/internal/store"
	"github.com/gorilla/mux"
	"github.com/rs/cors"
)

func main() {
	// Parse command line flags
	var (
		port        = flag.String("port", "8080", "Server port")
		dbPath      = flag.String("db", "./data/blackjack.db", "Database path")
		frontendURL = flag.String("frontend", "http://localhost:5173", "Frontend URL for CORS")
	)
	flag.Parse()

	// Create data directory if it doesn't exist
	dataDir := filepath.Dir(*dbPath)
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		log.Fatalf("Failed to create data directory: %v", err)
	}

	// Initialize the store
	gameStore := store.NewMemoryStore()
	log.Println("In-memory game store initialized")

	// Initialize the database
	database, err := db.NewDatabase(*dbPath)
	if err != nil {
		log.Printf("Warning: Failed to initialize database: %v", err)
		log.Println("Continuing without database persistence")
		database = nil
	} else {
		log.Println("Database initialized successfully")
		defer database.Close()
	}

	// Initialize WebSocket hub
	hub := api.NewHub()
	go hub.Run()
	log.Println("WebSocket hub started")

	// Initialize API handlers
	handlers := api.NewHandlers(gameStore, database, hub)

	// Set up router
	r := mux.NewRouter()
	handlers.RegisterRoutes(r)

	// Add middleware for logging
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			next.ServeHTTP(w, r)
			log.Printf("%s %s %s", r.Method, r.RequestURI, time.Since(start))
		})
	})

	// Configure CORS
	c := cors.New(cors.Options{
		AllowedOrigins:   []string{*frontendURL},
		AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "Authorization"},
		AllowCredentials: true,
	})

	// Create server
	srv := &http.Server{
		Addr:         ":" + *port,
		Handler:      c.Handler(r),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in a goroutine
	go func() {
		log.Printf("Starting server on port %s", *port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Set up graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// Block until we receive a termination signal
	<-stop

	log.Println("Shutting down server...")
}
