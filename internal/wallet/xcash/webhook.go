package xcash

import (
	"crypto/hmac"
	"encoding/json"
	"fmt"
)

type WebhookValidator struct {
	hmacKey string
}

func NewWebhookValidator(hmacKey string) *WebhookValidator {
	return &WebhookValidator{hmacKey: hmacKey}
}

func (v *WebhookValidator) Validate(body []byte, headers map[string]string) (*DepositWebhook, error) {
	nonce := headers["XC-Nonce"]
	timestamp := headers["XC-Timestamp"]
	signature := headers["XC-Signature"]

	expected := sign(nonce+timestamp+string(body), v.hmacKey)
	if !hmac.Equal([]byte(signature), []byte(expected)) {
		return nil, fmt.Errorf("invalid webhook signature")
	}

	var webhook DepositWebhook
	if err := json.Unmarshal(body, &webhook); err != nil {
		return nil, fmt.Errorf("unmarshal webhook: %w", err)
	}
	return &webhook, nil
}
