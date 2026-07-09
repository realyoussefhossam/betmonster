package xcash

import (
	"context"
	"crypto/hmac"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/go-redis/redis/v8"
)

const (
	defaultWebhookWindow = 5 * time.Minute
	webhookNoncePrefix   = "xcash:webhook:nonce:"
	futureTolerance      = 30 * time.Second
)

type WebhookValidator struct {
	hmacKey string
	redis   redis.Cmdable
	window  time.Duration
}

func NewWebhookValidator(hmacKey string) *WebhookValidator {
	return &WebhookValidator{
		hmacKey: hmacKey,
		window:  defaultWebhookWindow,
	}
}

func (v *WebhookValidator) WithRedis(r redis.Cmdable) *WebhookValidator {
	v.redis = r
	return v
}

func (v *WebhookValidator) Validate(body []byte, headers map[string]string) (*DepositWebhook, error) {
	nonce := headers["XC-Nonce"]
	timestamp := headers["XC-Timestamp"]
	signature := headers["XC-Signature"]

	expected := Sign(nonce+timestamp+string(body), v.hmacKey)
	if !hmac.Equal([]byte(signature), []byte(expected)) {
		return nil, fmt.Errorf("invalid webhook signature")
	}

	if err := v.validateTimestamp(timestamp); err != nil {
		return nil, err
	}

	if v.redis != nil {
		if err := v.checkNonce(context.Background(), nonce, timestamp); err != nil {
			return nil, err
		}
	}

	var webhook DepositWebhook
	if err := json.Unmarshal(body, &webhook); err != nil {
		return nil, fmt.Errorf("unmarshal webhook: %w", err)
	}
	return &webhook, nil
}

func (v *WebhookValidator) validateTimestamp(timestamp string) error {
	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid webhook timestamp: %w", err)
	}
	webhookTime := time.Unix(ts, 0)
	now := time.Now()
	if webhookTime.Before(now.Add(-v.window)) {
		return fmt.Errorf("webhook timestamp outside acceptable window")
	}
	if webhookTime.After(now.Add(futureTolerance)) {
		return fmt.Errorf("webhook timestamp too far in the future")
	}
	return nil
}

func (v *WebhookValidator) checkNonce(ctx context.Context, nonce, timestamp string) error {
	key := webhookNoncePrefix + nonce
	ok, err := v.redis.SetNX(ctx, key, timestamp, v.window).Result()
	if err != nil {
		return fmt.Errorf("webhook nonce check failed: %w", err)
	}
	if !ok {
		return fmt.Errorf("webhook nonce already used")
	}
	return nil
}
