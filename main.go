package main

import (
	"context"
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/vitorvezani/rinha-de-backend-2025-go/pkg/handlers"
	"github.com/vitorvezani/rinha-de-backend-2025-go/pkg/processor"
)

func main() {
	redisHost := os.Getenv("REDIS_HOST")
	if redisHost == "" {
		redisHost = "redis"
	}

	redisPort := os.Getenv("REDIS_PORT")
	if redisPort == "" {
		redisPort = "6379"
	}

	rdb := redis.NewClient(&redis.Options{
		Addr: redisHost + ":" + redisPort,
		DB:   0,
	})

	ctx := context.Background()
	_, err := rdb.Ping(ctx).Result()
	if err != nil {
		log.Fatal("Failed to connect to Redis:", err)
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
	router.POST("/payments", handlers.HandlePaymentProcessorAsync(rdb, defaultProcessor, fallbackProcessor))
	router.GET("/payments-summary", handlers.HandlePaymentsSummary(rdb))
	router.POST("/purge-payments", handlers.HandlePurgePayments(rdb))
	router.Run(":" + port)
}
