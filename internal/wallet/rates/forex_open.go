package rates

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

const openExchangeDefaultURL = "https://open.er-api.com/v6/latest/USD"

type OpenExchange struct {
	client *http.Client
	url    string
}

type OpenExchangeOption func(*OpenExchange)

func WithOpenExchangeURL(u string) OpenExchangeOption {
	return func(o *OpenExchange) { o.url = u }
}

func NewOpenExchange(opts ...OpenExchangeOption) *OpenExchange {
	url := os.Getenv("FOREX_API_URL")
	if url == "" {
		url = openExchangeDefaultURL
	}
	o := &OpenExchange{
		client: &http.Client{Timeout: 5 * time.Second},
		url:    url,
	}
	for _, opt := range opts {
		opt(o)
	}
	return o
}

func (o *OpenExchange) Name() string { return "open-er-api" }

func (o *OpenExchange) GetRate(ctx context.Context, fiat string) (string, error) {
	fiat = strings.ToUpper(fiat)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, o.url, nil)
	if err != nil {
		return "", err
	}
	resp, err := o.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("open-er-api status %d", resp.StatusCode)
	}
	var result struct {
		Rates map[string]string `json:"rates"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	rate, ok := result.Rates[fiat]
	if !ok {
		return "", fmt.Errorf("open-er-api missing rate for %s", fiat)
	}
	return strings.TrimSpace(rate), nil
}
