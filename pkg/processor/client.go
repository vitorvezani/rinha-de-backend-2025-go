package processor

import (
	"sync"
	"time"
)

type Payment struct {
	CorrelationId string    `json:"correlationId"`
	Amount        float32   `json:"amount"`
	RequestedAt   time.Time `json:"requestedAt"`
}

type HealthResponse struct {
	Failing         bool `json:"failing"`
	MinResponseTime int  `json:"minResponseTime"`
}

type PaymentsSummary struct {
	TotalRequests     int64   `json:"totalRequests"`
	TotalAmount       float64 `json:"totalAmount"`
	TotalFee          float64 `json:"totalFee"`
	FeePerTransaction float64 `json:"feePerTransaction"`
}

type PaymentProcessor struct {
	Name        string
	Client      PaymentClient
	isAvailable bool

	mu sync.Mutex
}

func (pp *PaymentProcessor) setAvailable(available bool) {
	pp.mu.Lock()
	defer pp.mu.Unlock()

	pp.isAvailable = available
}

func (pp *PaymentProcessor) IsAvailable() bool {
	pp.mu.Lock()
	defer pp.mu.Unlock()

	return pp.isAvailable
}

type PaymentClient interface {
	MakePayment(payment Payment) (string, error)
	GetHealth() (*HealthResponse, error)
	GetPayment(id int64) (*Payment, error)
	GetAdminPaymentsSummary(from, to time.Time) (*PaymentsSummary, error)
	SetAdminConfigToken(token string) error
	SetAdminConfigDelay(delay int64) error
	SetAdminConfigFailure(failure bool) error
	SetAdminPurgePayments() error
}
