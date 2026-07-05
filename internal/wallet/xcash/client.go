package xcash

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/google/uuid"
)

type Client struct {
	baseURL string
	appID   string
	hmacKey string
	http    *http.Client
}

func NewClient(baseURL, appID, hmacKey string) *Client {
	return &Client{
		baseURL: baseURL,
		appID:   appID,
		hmacKey: hmacKey,
		http:    &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *Client) GetDepositAddress(ctx context.Context, req DepositAddressRequest) (*DepositAddressResponse, error) {
	u, err := url.Parse(c.baseURL + "/v1/deposit/address")
	if err != nil {
		return nil, fmt.Errorf("parse url: %w", err)
	}
	q := u.Query()
	q.Set("uid", req.UID)
	q.Set("chain", req.Chain)
	q.Set("crypto", req.Crypto)
	u.RawQuery = q.Encode()

	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	nonce := uuid.NewString()
	// Per xcash docs: GET requests have no body, so the signed payload is nonce + timestamp + "".
	signature := Sign(nonce+timestamp+"", c.hmacKey)

	hreq, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	hreq.Header.Set("XC-Appid", c.appID)
	hreq.Header.Set("XC-Timestamp", timestamp)
	hreq.Header.Set("XC-Nonce", nonce)
	hreq.Header.Set("XC-Signature", signature)

	resp, err := c.http.Do(hreq)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("xcash %d: %s", resp.StatusCode, body)
	}

	var result DepositAddressResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &result, nil
}

func Sign(message, key string) string {
	h := hmac.New(sha256.New, []byte(key))
	h.Write([]byte(message))
	return hex.EncodeToString(h.Sum(nil))
}
