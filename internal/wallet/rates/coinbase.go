package rates

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const coinbaseDefaultURL = "https://api.coinbase.com"

type Coinbase struct {
	client *http.Client
	url    string
}

type CoinbaseOption func(*Coinbase)

func WithCoinbaseURL(u string) CoinbaseOption {
	return func(c *Coinbase) {
		c.url = u
	}
}

func NewCoinbase(opts ...CoinbaseOption) *Coinbase {
	c := &Coinbase{
		client: &http.Client{Timeout: 5 * time.Second},
		url:    coinbaseDefaultURL,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

func (c *Coinbase) Name() string { return "coinbase" }

func (c *Coinbase) GetRate(ctx context.Context, fiat, crypto string) (string, error) {
	fiat = strings.ToUpper(fiat)
	crypto = strings.ToUpper(crypto)
	if fiat == "USD" && isStablecoin(crypto) {
		return "1.00", nil
	}
	crypto = normalizeSymbol(crypto)
	if fiat == "USD" {
		fiat = "USDT"
	}
	url := fmt.Sprintf("%s/v2/exchange-rates?currency=%s", c.url, crypto)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("coinbase status %d", resp.StatusCode)
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
		return "", fmt.Errorf("coinbase missing rate for %s", fiat)
	}
	return strings.TrimSpace(rate), nil
}
