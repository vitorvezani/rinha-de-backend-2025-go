package main

import (
	"database/sql"
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/vitorvezani/rinha-de-backend-2025-go/pkg/handlers"
	"github.com/vitorvezani/rinha-de-backend-2025-go/pkg/processor"
	_ "modernc.org/sqlite"
)

func main() {
	db, err := sql.Open("sqlite", "file:payments.db?mode=memory&cache=shared")
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
	_, err := db.Exec("CREATE TABLE payments (correlation_id TEXT PRIMARY KEY, processor TEXT, amount_in_cents INTEGER, created_at TIME WITHOUT TIMEZONE)")
	if err != nil {
		return err
	}
	_, err = db.Exec("CREATE INDEX idx_payments_processor_created_at ON payments (processor, created_at)")
	if err != nil {
		return err
	}

	return nil
}
