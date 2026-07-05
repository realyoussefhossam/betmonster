package gateway

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/realyoussefhossam/betmonster/internal/auth"
	pb "github.com/realyoussefhossam/betmonster/internal/proto"
	wallet "github.com/realyoussefhossam/betmonster/internal/wallet"
	"github.com/realyoussefhossam/betmonster/internal/wallet/rates"
	"github.com/realyoussefhossam/betmonster/internal/wallet/xcash"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"
)

func TestHealthHandler(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	srv := NewServer(logger, nil, nil, NewRateLimiter("memory", "", 100, 100), "", "", "USDT", "anvil", "", Limits{})
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "healthy")
}

func TestSupportedOptions(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	srv := NewServer(logger, nil, nil, NewRateLimiter("memory", "", 100, 100), "", "", "USDT,USDC", "anvil,base", "", Limits{})
	req := httptest.NewRequest(http.MethodGet, "/api/wallet/supported", nil)
	w := httptest.NewRecorder()

	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "USDT")
	assert.Contains(t, w.Body.String(), "USDC")
	assert.Contains(t, w.Body.String(), "anvil")
	assert.Contains(t, w.Body.String(), "base")
}

func TestHandleBalanceUnsupportedCurrency(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	srv := NewServer(logger, nil, nil, NewRateLimiter("memory", "", 100, 100), "", "", "USDT", "anvil", "", Limits{})
	req := httptest.NewRequest(http.MethodGet, "/api/wallet/balance?currency=BTC", nil)
	req = req.WithContext(context.WithValue(req.Context(), UserContextKey, auth.User{ID: "user-1"}))
	w := httptest.NewRecorder()

	srv.handleBalance(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "unsupported currency")
}

func TestHandleDepositAddressUnsupportedPair(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	srv := NewServer(logger, nil, nil, NewRateLimiter("memory", "", 100, 100), "", "", "USDT", "anvil", "USDT:anvil", Limits{})
	req := httptest.NewRequest(http.MethodGet, "/api/wallet/deposit-address?currency=BNB&chain=anvil", nil)
	req = req.WithContext(context.WithValue(req.Context(), UserContextKey, auth.User{ID: "user-1"}))
	w := httptest.NewRecorder()

	srv.handleDepositAddress(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "unsupported currency-chain pair")
}

func TestHandleXcashWebhookParsesNestedAmount(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	// Spin up a local wallet gRPC server so the gateway can forward webhooks.
	store := wallet.NewInMemoryStore()
	validator := xcash.NewWebhookValidator("hmac-key")
	svc := wallet.NewService(store, nil, validator, []string{"USDT:anvil"})
	grpcServer := grpc.NewServer()
	pb.RegisterWalletServiceServer(grpcServer, wallet.NewGRPCServer(svc, nil))

	listener := bufconn.Listen(1024)
	go func() {
		if err := grpcServer.Serve(listener); err != nil {
			t.Logf("grpc server error: %v", err)
		}
	}()
	defer grpcServer.Stop()

	conn, err := grpc.Dial("bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
		return listener.Dial()
	}), grpc.WithInsecure())
	assert.NoError(t, err)
	defer conn.Close()

	walletClient := &WalletClient{conn: pb.NewWalletServiceClient(conn)}
	srv := NewServer(logger, walletClient, nil, NewRateLimiter("memory", "", 100, 100), "", "", "USDT", "anvil", "USDT:anvil", Limits{})

	body := `{"type":"deposit","data":{"sys_no":"DXC1","uid":"u1","amount":"10","crypto":"USDT","chain":"anvil","confirmed":true,"hash":"0xabc","block":1,"risk_level":null,"risk_score":null}}`
	sig := xcash.Sign("nonce"+"1234567890"+body, "hmac-key")
	req := httptest.NewRequest(http.MethodPost, "/webhooks/xcash/deposit", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("XC-Nonce", "nonce")
	req.Header.Set("XC-Timestamp", "1234567890")
	req.Header.Set("XC-Signature", sig)
	w := httptest.NewRecorder()

	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "ok", w.Body.String())
}

func TestHandleRatesPublicEndpoint(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	// Spin up a local wallet gRPC server with a rates aggregator.
	store := wallet.NewInMemoryStore()
	validator := xcash.NewWebhookValidator("hmac-key")
	svc := wallet.NewService(store, nil, validator, []string{"USDT:anvil"})
	agg := rates.NewAggregator(rates.NewCache(time.Hour), rates.NewForexChain())
	grpcServer := grpc.NewServer()
	pb.RegisterWalletServiceServer(grpcServer, wallet.NewGRPCServer(svc, agg))

	listener := bufconn.Listen(1024)
	go func() {
		if err := grpcServer.Serve(listener); err != nil {
			t.Logf("grpc server error: %v", err)
		}
	}()
	defer grpcServer.Stop()

	conn, err := grpc.Dial("bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
		return listener.Dial()
	}), grpc.WithInsecure())
	assert.NoError(t, err)
	defer conn.Close()

	walletClient := &WalletClient{conn: pb.NewWalletServiceClient(conn)}
	srv := NewServer(logger, walletClient, nil, NewRateLimiter("memory", "", 100, 100), "", "", "USDT", "anvil", "USDT:anvil", Limits{})

	req := httptest.NewRequest(http.MethodGet, "/api/wallet/rates", nil)
	w := httptest.NewRecorder()

	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "fiat_currency")
	assert.Contains(t, w.Body.String(), "USD")
	assert.Contains(t, w.Body.String(), "rates")
	assert.Contains(t, w.Body.String(), "USDT")
}
