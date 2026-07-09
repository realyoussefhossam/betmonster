package xcash

import (
	"strconv"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
)

func newRedis(t *testing.T) *redis.Client {
	s := miniredis.RunT(t)
	t.Cleanup(func() { s.Close() })
	return redis.NewClient(&redis.Options{Addr: s.Addr()})
}

func makeSignedWebhook(t *testing.T, body, nonce, timestamp string, key string) map[string]string {
	t.Helper()
	return map[string]string{
		"XC-Nonce":     nonce,
		"XC-Timestamp": timestamp,
		"XC-Signature": Sign(nonce+timestamp+body, key),
	}
}

func TestWebhookValid(t *testing.T) {
	body := `{"type":"deposit","data":{"sys_no":"DXC1","uid":"u1","amount":"10","crypto":"USDT","chain":"base","confirmed":true,"hash":"0xabc","block":1,"risk_level":null,"risk_score":null}}`
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	headers := makeSignedWebhook(t, body, "n1", timestamp, "key")

	rdb := newRedis(t)
	validator := NewWebhookValidator("key").WithRedis(rdb)

	webhook, err := validator.Validate([]byte(body), headers)
	assert.NoError(t, err)
	assert.Equal(t, "DXC1", webhook.Data.SysNo)
	assert.Equal(t, "10", webhook.Data.Amount)
	assert.True(t, webhook.Data.Confirmed)
}

func TestWebhookReplayNonce(t *testing.T) {
	body := `{"type":"deposit","data":{"sys_no":"DXC1","uid":"u1","amount":"10","crypto":"USDT","chain":"base","confirmed":true,"hash":"0xabc","block":1,"risk_level":null,"risk_score":null}}`
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	headers := makeSignedWebhook(t, body, "n1", timestamp, "key")

	rdb := newRedis(t)
	validator := NewWebhookValidator("key").WithRedis(rdb)

	_, err := validator.Validate([]byte(body), headers)
	assert.NoError(t, err)

	_, err = validator.Validate([]byte(body), headers)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "nonce already used")
}

func TestWebhookOldTimestamp(t *testing.T) {
	body := `{"type":"deposit"}`
	oldTime := strconv.FormatInt(time.Now().Add(-10*time.Minute).Unix(), 10)
	headers := makeSignedWebhook(t, body, "n1", oldTime, "key")

	rdb := newRedis(t)
	validator := NewWebhookValidator("key").WithRedis(rdb)

	_, err := validator.Validate([]byte(body), headers)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "timestamp")
}

func TestWebhookFutureTimestamp(t *testing.T) {
	body := `{"type":"deposit"}`
	futureTime := strconv.FormatInt(time.Now().Add(2*time.Minute).Unix(), 10)
	headers := makeSignedWebhook(t, body, "n1", futureTime, "key")

	rdb := newRedis(t)
	validator := NewWebhookValidator("key").WithRedis(rdb)

	_, err := validator.Validate([]byte(body), headers)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "timestamp")
}

func TestWebhookWithoutRedis(t *testing.T) {
	body := `{"type":"deposit","data":{"sys_no":"DXC1","uid":"u1","amount":"10","crypto":"USDT","chain":"base","confirmed":true,"hash":"0xabc","block":1,"risk_level":null,"risk_score":null}}`
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	headers := makeSignedWebhook(t, body, "n1", timestamp, "key")

	validator := NewWebhookValidator("key")

	webhook, err := validator.Validate([]byte(body), headers)
	assert.NoError(t, err)
	assert.Equal(t, "DXC1", webhook.Data.SysNo)

	// Without Redis, the same nonce is accepted again (test-only behaviour for backward compatibility).
	_, err = validator.Validate([]byte(body), headers)
	assert.NoError(t, err)
}

func TestWebhookInvalidSignature(t *testing.T) {
	body := `{"type":"deposit"}`
	validator := NewWebhookValidator("key")
	headers := map[string]string{
		"XC-Nonce":     "n1",
		"XC-Timestamp": strconv.FormatInt(time.Now().Unix(), 10),
		"XC-Signature": "bad-signature",
	}
	_, err := validator.Validate([]byte(body), headers)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "signature")
}
