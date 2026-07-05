package rates

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const moneyConvertDefaultURL = "https://cdn.moneyconvert.net/api/latest.json"

type MoneyConvert struct {
	client *http.Client
	url    string
}

type MoneyConvertOption func(*MoneyConvert)

func WithMoneyConvertURL(u string) MoneyConvertOption {
	return func(m *MoneyConvert) { m.url = u }
}

func NewMoneyConvert(opts ...MoneyConvertOption) *MoneyConvert {
	m := &MoneyConvert{
		client: &http.Client{Timeout: 5 * time.Second},
		url:    moneyConvertDefaultURL,
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

func (m *MoneyConvert) Name() string { return "moneyconvert" }

func (m *MoneyConvert) GetRate(ctx context.Context, fiat string) (string, error) {
	fiat = strings.ToUpper(fiat)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, m.url, nil)
	if err != nil {
		return "", err
	}
	resp, err := m.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("moneyconvert status %d", resp.StatusCode)
	}
	var result struct {
		Rates map[string]string `json:"rates"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	rate, ok := result.Rates[fiat]
	if !ok {
		return "", fmt.Errorf("moneyconvert missing rate for %s", fiat)
	}
	return strings.TrimSpace(rate), nil
}
