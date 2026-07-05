package rates

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const fawazAhmed0DefaultURL = "https://cdn.jsdelivr.net/npm/@fawazahmed0/currency-api@latest/v1/currencies/usd.json"

type FawazAhmed0 struct {
	client *http.Client
	url    string
}

type FawazAhmed0Option func(*FawazAhmed0)

func WithFawazAhmed0URL(u string) FawazAhmed0Option {
	return func(f *FawazAhmed0) { f.url = u }
}

func NewFawazAhmed0(opts ...FawazAhmed0Option) *FawazAhmed0 {
	f := &FawazAhmed0{
		client: &http.Client{Timeout: 5 * time.Second},
		url:    fawazAhmed0DefaultURL,
	}
	for _, opt := range opts {
		opt(f)
	}
	return f
}

func (f *FawazAhmed0) Name() string { return "fawazahmed0" }

func (f *FawazAhmed0) GetRate(ctx context.Context, fiat string) (string, error) {
	fiat = strings.ToLower(fiat)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, f.url, nil)
	if err != nil {
		return "", err
	}
	resp, err := f.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fawazahmed0 status %d", resp.StatusCode)
	}
	var result struct {
		USD map[string]json.RawMessage `json:"usd"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	raw, ok := result.USD[fiat]
	if !ok {
		return "", fmt.Errorf("fawazahmed0 missing rate for %s", fiat)
	}
	var rate string
	if err := json.Unmarshal(raw, &rate); err != nil {
		return "", err
	}
	return strings.TrimSpace(rate), nil
}
