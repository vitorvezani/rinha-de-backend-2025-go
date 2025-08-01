package processor

import (
	"context"
	"fmt"
	"time"

	"github.com/go-resty/resty/v2"
)

// RestClient implements PaymentProcessor using resty
type RestClient struct {
	client  *resty.Client
	baseURL string
	token   string
}

// NewClient constructs a RestClient
func NewClient(baseURL string) PaymentClient {
	r := resty.New()
	return &RestClient{
		client:  r,
		baseURL: baseURL,
	}
}

// NewClient constructs a PaymentClient
func NewPaymentProcessor(name string, client PaymentClient) *PaymentProcessor {
	return &PaymentProcessor{
		Name:   name,
		Client: client,
		info: Info{
			IsAvailable:     true,
			MinResponseTime: 0,
		},
	}
}

// SetAuthToken updates the Authorization header for admin actions
func (rc *RestClient) SetAuthToken(token string) {
	rc.token = token
	rc.client.SetAuthToken(token)
}

// MakePayment sends a payment request
func (rc *RestClient) MakePayment(ctx context.Context, payment Payment) (string, error) {
	var resp struct {
		Message string `json:"message"`
	}
	response, err := rc.client.R().
		SetContext(ctx).
		SetBody(payment).
		SetResult(&resp).
		Post(fmt.Sprintf("%s/payments", rc.baseURL))
	if err != nil {
		return "", err
	}

	if response.IsError() {
		return "", fmt.Errorf("error from MakePayment: %s", response.Status())
	}

	return resp.Message, nil
}

// GetHealth checks service health
func (rc *RestClient) GetHealth(ctx context.Context) (*HealthResponse, error) {
	var health HealthResponse
	response, err := rc.client.R().
		SetContext(ctx).
		SetResult(&health).
		Get(fmt.Sprintf("%s/payments/service-health", rc.baseURL))

	if response.IsError() {
		return nil, fmt.Errorf("error from GetHealth: %s", response.Status())
	}
	return &health, err
}

// GetPayment retrieves a payment by ID
func (rc *RestClient) GetPayment(ctx context.Context, id int64) (*Payment, error) {
	var payment Payment
	response, err := rc.client.R().
		SetContext(ctx).
		SetResult(&payment).
		Get(fmt.Sprintf("%s/payments/%d", rc.baseURL, id))
	if response.IsError() {
		return nil, fmt.Errorf("error from GetPayment: %s", response.Status())
	}
	return &payment, err
}

// GetAdminPaymentsSummary fetches payments summary between dates
func (rc *RestClient) GetAdminPaymentsSummary(ctx context.Context, from, to time.Time) (*PaymentsSummary, error) {
	var summary PaymentsSummary
	response, err := rc.client.R().
		SetContext(ctx).
		SetQueryParams(map[string]string{
			"from": from.Format(time.RFC3339),
			"to":   to.Format(time.RFC3339),
		}).
		SetResult(&summary).
		Get(fmt.Sprintf("%s/admin/payments-summary", rc.baseURL))

	if response.IsError() {
		return nil, fmt.Errorf("error from GetAdminPaymentsSummary: %s", response.Status())
	}
	return &summary, err
}

// SetAdminConfigToken sets admin token
func (rc *RestClient) SetAdminConfigToken(ctx context.Context, token string) error {
	body := map[string]string{"token": token}
	response, err := rc.client.R().
		SetContext(ctx).
		SetBody(body).
		Post(fmt.Sprintf("%s/admin/config/token", rc.baseURL))

	if response.IsError() {
		return fmt.Errorf("error from SetAdminConfigToken: %s", response.Status())
	}

	return err
}

// SetAdminConfigDelay sets admin delay
func (rc *RestClient) SetAdminConfigDelay(ctx context.Context, delay int64) error {
	body := map[string]int64{"delay": delay}
	response, err := rc.client.R().
		SetContext(ctx).
		SetBody(body).
		Post(fmt.Sprintf("%s/admin/config/delay", rc.baseURL))
	if response.IsError() {
		return fmt.Errorf("error from SetAdminConfigDelay: %s", response.Status())
	}
	return err
}

// SetAdminConfigFailure toggles admin failure
func (rc *RestClient) SetAdminConfigFailure(ctx context.Context, failure bool) error {
	body := map[string]bool{"failure": failure}
	response, err := rc.client.R().
		SetContext(ctx).
		SetBody(body).
		Post(fmt.Sprintf("%s/admin/config/failure", rc.baseURL))

	if response.IsError() {
		return fmt.Errorf("error from SetAdminConfigFailure: %s", response.Status())
	}

	return err
}

// SetAdminPurgePayments purges all payments
func (rc *RestClient) SetAdminPurgePayments(ctx context.Context) error {
	response, err := rc.client.R().
		SetContext(ctx).
		Post(fmt.Sprintf("%s/admin/payments", rc.baseURL))

	if response.IsError() {
		return fmt.Errorf("error from SetAdminPurgePayments: %s", response.Status())
	}

	return err
}
