package xcash

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestClientGetDepositAddress(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/deposit/address", r.URL.Path)
		assert.Equal(t, "user-1", r.URL.Query().Get("uid"))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"deposit_address":"0x123"}`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "appid", "key")
	resp, err := client.GetDepositAddress(context.Background(), DepositAddressRequest{UID: "user-1", Chain: "base", Crypto: "USDT"})
	assert.NoError(t, err)
	assert.Equal(t, "0x123", resp.Address)
}

func TestWebhookValidator(t *testing.T) {
	body := `{"type":"deposit","data":{"sys_no":"DXC1","uid":"u1","amount":"10","crypto":"USDT","chain":"base","confirmed":true,"hash":"0xabc","block":1,"risk_level":null,"risk_score":null}}`
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	validator := NewWebhookValidator("key")
	headers := map[string]string{
		"XC-Nonce":     "n1",
		"XC-Timestamp": timestamp,
		"XC-Signature": Sign("n1"+timestamp+body, "key"),
	}
	webhook, err := validator.Validate(context.Background(), []byte(body), headers)
	assert.NoError(t, err)
	assert.Equal(t, "DXC1", webhook.Data.SysNo)
	assert.Equal(t, "10", webhook.Data.Amount)
	assert.True(t, webhook.Data.Confirmed)
}

func TestWebhookValidatorInvalidSignature(t *testing.T) {
	body := `{"type":"deposit"}`
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	validator := NewWebhookValidator("key")
	headers := map[string]string{
		"XC-Nonce":     "n1",
		"XC-Timestamp": timestamp,
		"XC-Signature": "bad-signature",
	}
	_, err := validator.Validate(context.Background(), []byte(body), headers)
	assert.Error(t, err)
}
