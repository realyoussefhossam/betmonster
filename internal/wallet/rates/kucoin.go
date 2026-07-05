package rates

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const kucoinDefaultURL = "https://api.kucoin.com"

type KuCoin struct {
	client *http.Client
	url    string
}

type KuCoinOption func(*KuCoin)

func WithKuCoinURL(u string) KuCoinOption {
	return func(k *KuCoin) {
		k.url = u
	}
}

func NewKuCoin(opts ...KuCoinOption) *KuCoin {
	k := &KuCoin{
		client: &http.Client{Timeout: 5 * time.Second},
		url:    kucoinDefaultURL,
	}
	for _, opt := range opts {
		opt(k)
	}
	return k
}

func (k *KuCoin) Name() string { return "kucoin" }

func (k *KuCoin) GetRate(ctx context.Context, fiat, crypto string) (string, error) {
	if fiat == "USD" && isStablecoin(crypto) {
		return "1.00", nil
	}
	crypto = normalizeSymbol(crypto)
	url := fmt.Sprintf("%s/api/v1/prices?base=%s&currencies=%s", k.url, fiat, crypto)
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
		return "", fmt.Errorf("kucoin status %d", resp.StatusCode)
	}
	var result struct {
		Code string            `json:"code"`
		Data map[string]string `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if result.Code != "200000" {
		return "", fmt.Errorf("kucoin code %s", result.Code)
	}
	price, ok := result.Data[crypto]
	if !ok {
		return "", fmt.Errorf("kucoin missing rate for %s", crypto)
	}
	return strings.TrimSpace(price), nil
}
