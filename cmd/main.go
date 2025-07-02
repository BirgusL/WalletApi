package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"WalletApi/internal/handler"
	"WalletApi/internal/repository"
	"WalletApi/internal/service"

	_ "github.com/lib/pq"
)

const (
	workers = 16 // The number of goroutines for processing transactions
)

func main() {
	// Checking required environment variables
	requiredEnvVars := []string{"DB_URL", "DB_NAME", "DB_USER", "DB_PASSWORD"}
	for _, envVar := range requiredEnvVars {
		if os.Getenv(envVar) == "" {
			log.Fatalf("Environment variable %s is not set", envVar)
		}
	}

	dbURL := os.Getenv("DB_URL")
	if dbURL == "" {
		log.Fatal("DB_URL environment variable is not set")
	}

	// Connecting to the database
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("Database connection failed: %v", err)
	}
	defer db.Close()

	// Configuring the Connection pool
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Checking the connection
	if err := db.Ping(); err != nil {
		log.Fatalf("Database ping failed: %v", err)
	}

	// Initializing the repository
	walletRepo := repository.NewPostgresRepository(db)

	if err := walletRepo.RunMigrations(context.Background()); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	// Initializing the service
	walletService := service.NewWalletService(walletRepo, workers)
	defer walletService.Shutdown() // Graceful shutdown сервиса

	// Initializing the handler
	walletHandler := handler.NewWalletHandler(walletService)

	// Setting up routes
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/wallets", walletHandler.CreateWallet)
	mux.HandleFunc("POST /api/v1/wallets/{id}/transactions", walletHandler.HandleTransaction)
	mux.HandleFunc("GET /api/v1/wallets/{id}", walletHandler.HandleGetBalance)

	// Starting the server
	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	go func() {
		log.Println("Server started on :8080")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown:", err)
	}
	log.Println("Server exiting")
}
