package rates

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const krakenDefaultURL = "https://api.kraken.com"

type Kraken struct {
	client *http.Client
	url    string
}

type KrakenOption func(*Kraken)

func WithKrakenURL(u string) KrakenOption {
	return func(k *Kraken) {
		k.url = u
	}
}

func NewKraken(opts ...KrakenOption) *Kraken {
	k := &Kraken{
		client: &http.Client{Timeout: 5 * time.Second},
		url:    krakenDefaultURL,
	}
	for _, opt := range opts {
		opt(k)
	}
	return k
}

func (k *Kraken) Name() string { return "kraken" }

func (k *Kraken) GetRate(ctx context.Context, fiat, crypto string) (string, error) {
	if fiat == "USD" && isStablecoin(crypto) {
		return "1.00", nil
	}
	crypto = normalizeSymbol(crypto)
	if fiat == "USD" {
		fiat = "USDT"
	}
	url := fmt.Sprintf("%s/0/public/Ticker?pair=%s%s", k.url, crypto, fiat)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := k.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("kraken status %d", resp.StatusCode)
	}
	var result struct {
		Error  []string               `json:"error"`
		Result map[string]interface{} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if len(result.Error) > 0 {
		return "", fmt.Errorf("kraken error: %v", result.Error)
	}
	for _, v := range result.Result {
		pair, ok := v.(map[string]interface{})
		if !ok {
			continue
		}
		c, ok := pair["c"].([]interface{})
		if !ok || len(c) == 0 {
			continue
		}
		price, ok := c[0].(string)
		if ok {
			return strings.TrimSpace(price), nil
		}
	}
	return "", fmt.Errorf("kraken missing ticker data")
}
