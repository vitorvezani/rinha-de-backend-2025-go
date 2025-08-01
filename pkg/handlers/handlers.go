package handlers

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/vitorvezani/rinha-de-backend-2025-go/pkg/processor"
)

func HandlePaymentsSummary(rdb *redis.Client) func(c *gin.Context) {
	return func(c *gin.Context) {
		fromStr := c.Query("from")
		toStr := c.Query("to")

		from, err := time.Parse(time.RFC3339, fromStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid 'from' date"})
			return
		}

		to, err := time.Parse(time.RFC3339, toStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid 'to' date"})
			return
		}

		fmt.Printf("Received handlePaymentsSummary from %s to %s\n", from, to)

		// Get all payment keys
		keys, err := rdb.Keys(c.Request.Context(), "payment:*").Result()
		if err != nil {
			log.Println("Error getting payment keys:", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}

		// Result map
		result := map[string]gin.H{
			"default": {
				"totalRequests": 0,
				"totalAmount":   float64(0),
			},
			"fallback": {
				"totalRequests": 0,
				"totalAmount":   float64(0),
			},
		}

		for _, key := range keys {
			paymentData, err := rdb.HGetAll(c.Request.Context(), key).Result()
			if err != nil {
				log.Println("Error getting payment data:", err)
				continue
			}

			createdAtStr, exists := paymentData["created_at"]
			if !exists {
				continue
			}

			createdAt, err := time.Parse(time.RFC3339, createdAtStr)
			if err != nil {
				continue
			}

			// Check if payment is within date range
			if createdAt.Before(from) || createdAt.After(to) {
				continue
			}

			processor := paymentData["processor"]
			amountStr := paymentData["amount_in_cents"]
			amount, err := strconv.ParseFloat(amountStr, 64)
			if err != nil {
				continue
			}

			if processorData, exists := result[processor]; exists {
				totalRequests := processorData["totalRequests"].(int) + 1
				totalAmount := processorData["totalAmount"].(float64) + (amount / 100.0)

				result[processor] = gin.H{
					"totalRequests": totalRequests,
					"totalAmount":   totalAmount,
				}
			}
		}

		c.JSON(http.StatusOK, result)
	}
}

func HandlePaymentProcessor(rdb *redis.Client, defaultProcessor, fallbackProcessor *processor.PaymentProcessor) func(c *gin.Context) {
	return func(c *gin.Context) {
		var body processor.Payment
		err := c.BindJSON(&body)
		if err != nil {
			log.Println("Error parsing body", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "error parsing body"})
			return
		}

		paymentRequest := processor.Payment{CorrelationId: body.CorrelationId, Amount: body.Amount, RequestedAt: time.Now()}
		var processorUsed string

		if defaultProcessor.GetInfo().IsAvailable {
			processorUsed = defaultProcessor.Name
			_, err = defaultProcessor.Client.MakePayment(c.Request.Context(), paymentRequest)
		} else {
			err = fmt.Errorf("default processor is not available")
		}
		if err != nil {
			log.Println("error in default processor, falling back to fallback processor", err)
			if fallbackProcessor.GetInfo().IsAvailable {
				processorUsed = fallbackProcessor.Name
				_, err = fallbackProcessor.Client.MakePayment(c.Request.Context(), paymentRequest)
				if err != nil {
					log.Println("error in fallback processor, erroring out", err)
					c.JSON(http.StatusFailedDependency, "could not make payment")
					return
				}
			} else {
				// put in a queue?
				log.Println("both processor are not available, can't do shit", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
				return
			}
		}

		paymentKey := fmt.Sprintf("payment:%s", body.CorrelationId)

		// Store payment data as a hash in Redis
		paymentData := map[string]interface{}{
			"correlation_id":  body.CorrelationId,
			"processor":       processorUsed,
			"amount_in_cents": body.Amount * 100.0,
			"created_at":      time.Now().Format(time.RFC3339),
		}

		err = rdb.HMSet(c.Request.Context(), paymentKey, paymentData).Err()
		if err != nil {
			log.Println("error storing payment in Redis", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}

		c.JSON(200, gin.H{
			"message": "Payment processed!",
		})
	}
}

func HandlePaymentProcessorAsync(rdb *redis.Client, defaultProcessor, fallbackProcessor *processor.PaymentProcessor) func(c *gin.Context) {
	return func(c *gin.Context) {
		var body processor.Payment
		err := c.BindJSON(&body)
		if err != nil {
			log.Println("Error parsing body", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "error parsing body"})
			return
		}

		paymentRequest := processor.Payment{
			CorrelationId: body.CorrelationId,
			Amount:        body.Amount,
			RequestedAt:   time.Now(),
		}

		// Return immediately to client
		c.JSON(http.StatusAccepted, gin.H{
			"message":       "Payment accepted for processing",
			"correlationId": body.CorrelationId,
		})

		// Process payment asynchronously
		go processPaymentAsync(rdb, paymentRequest, defaultProcessor, fallbackProcessor)
	}
}

func processPaymentAsync(rdb *redis.Client, paymentRequest processor.Payment, defaultProcessor, fallbackProcessor *processor.PaymentProcessor) {
	maxRetries := 5
	retryDelay := time.Second * 2

	for attempt := range maxRetries {
		var processorUsed string
		var err error

		// Try default processor first (lower fees)
		if defaultProcessor.GetInfo().IsAvailable {
			processorUsed = defaultProcessor.Name
			_, err = defaultProcessor.Client.MakePayment(context.Background(), paymentRequest)
			if err == nil {
				// Success with default processor
				storePaymentResult(rdb, paymentRequest, processorUsed)
				log.Printf("Payment %s processed successfully with default processor", paymentRequest.CorrelationId)
				return
			}
			log.Printf("Default processor failed for payment %s: %v", paymentRequest.CorrelationId, err)
		}

		// Try fallback processor
		if fallbackProcessor.GetInfo().IsAvailable {
			processorUsed = fallbackProcessor.Name
			_, err = fallbackProcessor.Client.MakePayment(context.Background(), paymentRequest)
			if err == nil {
				// Success with fallback processor
				storePaymentResult(rdb, paymentRequest, processorUsed)
				log.Printf("Payment %s processed successfully with fallback processor", paymentRequest.CorrelationId)
				return
			}
			log.Printf("Fallback processor failed for payment %s: %v", paymentRequest.CorrelationId, err)
		}

		// Both processors failed, log and retry
		log.Printf("Both processors failed for payment %s (attempt %d/%d)", paymentRequest.CorrelationId, attempt+1, maxRetries)

		if attempt < maxRetries-1 {
			time.Sleep(retryDelay)
			retryDelay *= 2 // Exponential backoff
		}
	}

	// All retries exhausted
	log.Printf("Payment %s failed after %d attempts - storing as failed", paymentRequest.CorrelationId, maxRetries)
	go processPaymentAsync(rdb, paymentRequest, defaultProcessor, fallbackProcessor)
}

func storePaymentResult(rdb *redis.Client, paymentRequest processor.Payment, processorUsed string) {
	paymentKey := fmt.Sprintf("payment:%s", paymentRequest.CorrelationId)

	paymentData := map[string]interface{}{
		"correlation_id":  paymentRequest.CorrelationId,
		"processor":       processorUsed,
		"amount_in_cents": paymentRequest.Amount * 100.0,
		"created_at":      time.Now().Format(time.RFC3339),
		"status":          "completed",
	}

	err := rdb.HMSet(context.Background(), paymentKey, paymentData).Err()
	if err != nil {
		log.Printf("Error storing payment %s in Redis: %v", paymentRequest.CorrelationId, err)
	}
}

func HandlePurgePayments(rdb *redis.Client) func(c *gin.Context) {
	return func(c *gin.Context) {
		// Get all payment keys
		keys, err := rdb.Keys(c.Request.Context(), "payment:*").Result()
		if err != nil {
			log.Println("Error getting payment keys for purge:", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}

		// Delete all payment keys if any exist
		if len(keys) > 0 {
			deleted, err := rdb.Del(c.Request.Context(), keys...).Result()
			if err != nil {
				log.Println("Error purging payments:", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
				return
			}
			log.Printf("Purged %d payment records", deleted)
		}

		c.JSON(200, gin.H{
			"message": "Payments purged!",
		})
	}
}
