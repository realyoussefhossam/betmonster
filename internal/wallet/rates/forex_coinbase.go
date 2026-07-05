package rates

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const coinbaseForexDefaultURL = "https://api.coinbase.com/v2/exchange-rates?currency=USD"

type CoinbaseForex struct {
	client *http.Client
	url    string
}

type CoinbaseForexOption func(*CoinbaseForex)

func WithCoinbaseForexURL(u string) CoinbaseForexOption {
	return func(c *CoinbaseForex) { c.url = u }
}

func NewCoinbaseForex(opts ...CoinbaseForexOption) *CoinbaseForex {
	c := &CoinbaseForex{
		client: &http.Client{Timeout: 5 * time.Second},
		url:    coinbaseForexDefaultURL,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

func (c *CoinbaseForex) Name() string { return "coinbase-forex" }

func (c *CoinbaseForex) GetRate(ctx context.Context, fiat string) (string, error) {
	fiat = strings.ToUpper(fiat)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url, nil)
	if err != nil {
		return "", err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("coinbase-forex status %d", resp.StatusCode)
	}
	var result struct {
		Data struct {
			Rates map[string]string `json:"rates"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	rate, ok := result.Data.Rates[fiat]
	if !ok {
		return "", fmt.Errorf("coinbase-forex missing rate for %s", fiat)
	}
	return strings.TrimSpace(rate), nil
}
