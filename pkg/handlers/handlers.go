package handlers

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/vitorvezani/rinha-de-backend-2025-go/pkg/processor"
)

func HandlePaymentsSummary(db *sql.DB) func(c *gin.Context) {
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

		query := `
			SELECT processor, COUNT(*) AS total_requests, SUM(amount_in_cents) AS total_amount
			FROM payments
			WHERE created_at BETWEEN $1 AND $2
			GROUP BY processor
		`

		rows, err := db.Query(query, from, to)
		if err != nil {
			log.Println("Error querying payments summary:", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}
		defer rows.Close()

		// Result map
		result := make(map[string]gin.H)

		for rows.Next() {
			var processor string
			var totalRequests int
			var totalAmountCents int64

			if err := rows.Scan(&processor, &totalRequests, &totalAmountCents); err != nil {
				log.Println("Error scanning row:", err)
				continue
			}

			// Convert cents to float (e.g., 123456 => 1234.56)
			result[processor] = gin.H{
				"totalRequests": totalRequests,
				"totalAmount":   float64(totalAmountCents) / 100.0,
			}
		}

		if len(result["default"]) == 0 {
			result["default"] = gin.H{
				"totalRequests": 0,
				"totalAmount":   float64(0),
			}
		}

		if len(result["fallback"]) == 0 {
			result["fallback"] = gin.H{
				"totalRequests": 0,
				"totalAmount":   float64(0),
			}
		}

		if err := rows.Err(); err != nil {
			log.Println("Rows error:", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}

		c.JSON(http.StatusOK, result)
	}
}

func HandlePaymentProcessor(db *sql.DB, defaultProcessor, fallbackProcessor *processor.PaymentProcessor) func(c *gin.Context) {
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

		_, err = db.Exec("INSERT INTO payments (correlation_id, processor, amount_in_cents, created_at) VALUES (? ,?, ? ,?)", body.CorrelationId, processorUsed, body.Amount*100.0, time.Now())
		if err != nil {
			log.Println("error inserting payment into the database", err)
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
