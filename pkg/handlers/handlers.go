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

		ctx := context.Background()

		// Get all payment keys
		keys, err := rdb.Keys(ctx, "payment:*").Result()
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
			paymentData, err := rdb.HGetAll(ctx, key).Result()
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

		if defaultProcessor.IsAvailable() {
			processorUsed = defaultProcessor.Name
			_, err = defaultProcessor.Client.MakePayment(paymentRequest)
		} else {
			err = fmt.Errorf("default processor is not available")
		}
		if err != nil {
			log.Println("error in default processor, falling back to fallback processor", err)
			if fallbackProcessor.IsAvailable() {
				processorUsed = fallbackProcessor.Name
				_, err = fallbackProcessor.Client.MakePayment(paymentRequest)
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

		ctx := context.Background()
		paymentKey := fmt.Sprintf("payment:%s", body.CorrelationId)

		// Store payment data as a hash in Redis
		paymentData := map[string]interface{}{
			"correlation_id":  body.CorrelationId,
			"processor":       processorUsed,
			"amount_in_cents": body.Amount * 100.0,
			"created_at":      time.Now().Format(time.RFC3339),
		}

		err = rdb.HMSet(ctx, paymentKey, paymentData).Err()
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

func HandlePurgePayments() func(c *gin.Context) {
	return func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "Payments purged!",
		})
	}
}
