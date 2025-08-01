package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
	"github.com/vitorvezani/rinha-de-backend-2025-go/pkg/handlers"
	"github.com/vitorvezani/rinha-de-backend-2025-go/pkg/processor"
)

func main() {
	dbHost := os.Getenv("DB_HOST")
	dbPort := os.Getenv("DB_PORT")
	dbUser := os.Getenv("DB_USER")
	dbPassword := os.Getenv("DB_PASSWORD")
	dbName := os.Getenv("DB_NAME")

	psqlInfo := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable", dbHost, dbPort, dbUser, dbPassword, dbName)
	db, err := sql.Open("postgres", psqlInfo)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	err = setupDatabase(db)
	if err != nil {
		log.Fatal(err)
	}

	port := os.Getenv("APP_PORT")
	if port == "" {
		port = "8080"
	}

	processorDefaultURL := os.Getenv("PROCESSOR_DEFAULT_URL")
	if processorDefaultURL == "" {
		log.Fatal("PROCESSOR_DEFAULT_URL is required")
	}

	processorFallbackURL := os.Getenv("PROCESSOR_FALLBACK_URL")
	if processorFallbackURL == "" {
		log.Fatal("PROCESSOR_FALLBACK_URL is required")
	}

	defaultProcessorClient := processor.NewClient(processorDefaultURL)
	fallbackProcessorClient := processor.NewClient(processorFallbackURL)

	defaultProcessor := processor.NewPaymentProcessor("default", defaultProcessorClient)
	fallbackProcessor := processor.NewPaymentProcessor("fallback", fallbackProcessorClient)

	go processor.InstallPaymentProcessorWatcher(defaultProcessor)
	go processor.InstallPaymentProcessorWatcher(fallbackProcessor)

	router := gin.Default()
	router.POST("/payments", handlers.HandlePaymentProcessor(db, defaultProcessor, fallbackProcessor))
	router.GET("/payments-summary", handlers.HandlePaymentsSummary(db))
	router.POST("/purge-payments", handlers.HandlePurgePayments())
	router.Run(":" + port)
}

func setupDatabase(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS payments (
	   correlation_id TEXT PRIMARY KEY,
	   processor TEXT,
	   amount_in_cents INTEGER,
	   created_at TIMESTAMP WITHOUT TIME ZONE
   )`)
	if err != nil {
		return err
	}
	_, err = db.Exec("CREATE INDEX IF NOT EXISTS idx_payments_processor_created_at ON payments (processor, created_at)")
	if err != nil {
		return err
	}
	return nil
}
