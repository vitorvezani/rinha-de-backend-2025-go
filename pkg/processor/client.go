package processor

import (
	"context"
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
	MakePayment(ctx context.Context, payment Payment) (string, error)
	GetHealth(ctx context.Context) (*HealthResponse, error)
	GetPayment(ctx context.Context, id int64) (*Payment, error)
	GetAdminPaymentsSummary(ctx context.Context, from, to time.Time) (*PaymentsSummary, error)
	SetAdminConfigToken(ctx context.Context, token string) error
	SetAdminConfigDelay(ctx context.Context, delay int64) error
	SetAdminConfigFailure(ctx context.Context, failure bool) error
	SetAdminPurgePayments(ctx context.Context) error
}
