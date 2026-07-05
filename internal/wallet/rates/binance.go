package rates

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const binanceDefaultURL = "https://api.binance.com"

type Binance struct {
	client *http.Client
	url    string
}

type BinanceOption func(*Binance)

func WithBinanceURL(u string) BinanceOption {
	return func(b *Binance) {
		b.url = u
	}
}

func NewBinance(opts ...BinanceOption) *Binance {
	b := &Binance{
		client: &http.Client{Timeout: 5 * time.Second},
		url:    binanceDefaultURL,
	}
	for _, opt := range opts {
		opt(b)
	}
	return b
}

func (b *Binance) Name() string { return "binance" }

func (b *Binance) GetRate(ctx context.Context, fiat, crypto string) (string, error) {
	if fiat == "USD" && isStablecoin(crypto) {
		return "1.00", nil
	}
	crypto = normalizeSymbol(crypto)
	if fiat == "USD" {
		fiat = "USDT"
	}
	url := fmt.Sprintf("%s/api/v3/ticker/price?symbol=%s%s", b.url, crypto, fiat)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := b.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("binance status %d", resp.StatusCode)
	}
	var result struct {
		Price string `json:"price"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if result.Price == "" {
		return "", fmt.Errorf("binance empty price")
	}
	return strings.TrimSpace(result.Price), nil
}
