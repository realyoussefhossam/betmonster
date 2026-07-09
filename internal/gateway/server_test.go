package gateway

import (
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/realyoussefhossam/betmonster/internal/auth"
	oddsfeed "github.com/realyoussefhossam/betmonster/internal/oddsfeed"
	pb "github.com/realyoussefhossam/betmonster/internal/proto"
	wallet "github.com/realyoussefhossam/betmonster/internal/wallet"
	"github.com/realyoussefhossam/betmonster/internal/wallet/rates"
	"github.com/realyoussefhossam/betmonster/internal/wallet/xcash"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

func TestHealthHandler(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	srv := NewServer(logger, nil, nil, nil, NewRateLimiter("memory", "", 100, 100), "", "", "USDT", "anvil", "", Limits{})
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "healthy")
}

func TestSupportedOptions(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	srv := NewServer(logger, nil, nil, nil, NewRateLimiter("memory", "", 100, 100), "", "", "USDT,USDC", "anvil,base", "", Limits{})
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
	srv := NewServer(logger, nil, nil, nil, NewRateLimiter("memory", "", 100, 100), "", "", "USDT", "anvil", "", Limits{})
	req := httptest.NewRequest(http.MethodGet, "/api/wallet/balance?currency=BTC", nil)
	req = req.WithContext(context.WithValue(req.Context(), UserContextKey, auth.User{ID: "user-1"}))
	w := httptest.NewRecorder()

	srv.handleBalance(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "unsupported currency")
}

func TestHandleDepositAddressUnsupportedPair(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	srv := NewServer(logger, nil, nil, nil, NewRateLimiter("memory", "", 100, 100), "", "", "USDT", "anvil", "USDT:anvil", Limits{})
	req := httptest.NewRequest(http.MethodGet, "/api/wallet/deposit-address?currency=BNB&chain=anvil", nil)
	req = req.WithContext(context.WithValue(req.Context(), UserContextKey, auth.User{ID: "user-1"}))
	w := httptest.NewRecorder()

	srv.handleDepositAddress(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "unsupported currency-chain pair")
}

func TestHandleXcashWebhookParsesNestedAmount(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	// Spin up a local wallet gRPC server with auth interceptor so the gateway's metadata is verified.
	store := wallet.NewInMemoryStore()
	validator := xcash.NewWebhookValidator("hmac-key")
	svc := wallet.NewService(store, nil, validator, []string{"USDT:anvil"})
	grpcServer := grpc.NewServer(grpc.UnaryInterceptor(wallet.AuthInterceptor))
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
	srv := NewServer(logger, walletClient, nil, nil, NewRateLimiter("memory", "", 100, 100), "", "", "USDT", "anvil", "USDT:anvil", Limits{})

	body := `{"type":"deposit","data":{"sys_no":"DXC1","uid":"u1","amount":"10","crypto":"USDT","chain":"anvil","confirmed":true,"hash":"0xabc","block":1,"risk_level":null,"risk_score":null}}`
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	sig := xcash.Sign("nonce"+timestamp+body, "hmac-key")
	req := httptest.NewRequest(http.MethodPost, "/webhooks/xcash/deposit", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("XC-Nonce", "nonce")
	req.Header.Set("XC-Timestamp", timestamp)
	req.Header.Set("XC-Signature", sig)
	w := httptest.NewRecorder()

	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "ok", w.Body.String())
}

func TestHandleRatesPublicEndpoint(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	// Spin up a local wallet gRPC server with auth interceptor so the gateway's metadata is verified.
	store := wallet.NewInMemoryStore()
	validator := xcash.NewWebhookValidator("hmac-key")
	svc := wallet.NewService(store, nil, validator, []string{"USDT:anvil"})
	agg := rates.NewAggregator(rates.NewCache(time.Hour), rates.NewForexChain())
	grpcServer := grpc.NewServer(grpc.UnaryInterceptor(wallet.AuthInterceptor))
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
	srv := NewServer(logger, walletClient, nil, nil, NewRateLimiter("memory", "", 100, 100), "", "", "USDT", "anvil", "USDT:anvil", Limits{})

	req := httptest.NewRequest(http.MethodGet, "/api/wallet/rates", nil)
	w := httptest.NewRecorder()

	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "fiat_currency")
	assert.Contains(t, w.Body.String(), "USD")
	assert.Contains(t, w.Body.String(), "rates")
	assert.Contains(t, w.Body.String(), "USDT")
}

func TestGatewayForwardsCallerIdentityToWallet(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	// Spin up a wallet gRPC server with the auth interceptor.
	store := wallet.NewInMemoryStore()
	_, err := store.CreditWallet(context.Background(), "user-42", "USDT", "100.00", "dx-1", nil)
	require.NoError(t, err)

	svc := wallet.NewService(store, nil, nil, []string{"USDT:anvil"})
	grpcServer := grpc.NewServer(grpc.UnaryInterceptor(wallet.AuthInterceptor))
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
	require.NoError(t, err)
	defer conn.Close()

	walletClient := &WalletClient{conn: pb.NewWalletServiceClient(conn)}
	srv := NewServer(logger, walletClient, nil, nil, NewRateLimiter("memory", "", 100, 100), "", "", "USDT", "anvil", "USDT:anvil", Limits{})

	req := httptest.NewRequest(http.MethodGet, "/api/wallet/balance", nil)
	req = req.WithContext(context.WithValue(req.Context(), UserContextKey, auth.User{ID: "user-42"}))
	w := httptest.NewRecorder()

	// Call the handler directly so we do not need a real JWKS client.
	srv.handleBalance(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "100")
}

func newOddsFeedServer(t *testing.T) (oddsfeed.Store, *bufconn.Listener, *grpc.Server) {
	t.Helper()
	ctx := context.Background()
	store := oddsfeed.NewInMemoryStore()
	svc := oddsfeed.NewService(store, nil, nil, nil, nil)

	_, err := store.UpsertSport(ctx, oddsfeed.Sport{
		ID: "sport-1", Name: "Soccer", Slug: "soccer", Provider: "mock", ProviderSportID: "sp-1",
	})
	require.NoError(t, err)
	_, err = store.UpsertLeague(ctx, oddsfeed.League{
		ID: "league-1", Name: "Mock League", SportID: "sport-1", Provider: "mock", ProviderLeagueID: "lg-1", Country: "Mockland",
	})
	require.NoError(t, err)
	_, err = store.UpsertEvent(ctx, oddsfeed.Event{
		ID: "event-1", LeagueID: "league-1", SportID: "sport-1",
		HomeParticipant: "Mock FC", AwayParticipant: "Test United",
		Status: "upcoming", StartsAt: time.Now().Add(2 * time.Hour),
		Provider: "mock", ProviderEventID: "ev-1",
	})
	require.NoError(t, err)
	_, err = store.UpsertMarket(ctx, oddsfeed.Market{
		ID: "market-1", EventID: "event-1", Type: "1x2", Name: "Match Result", Status: "active",
		Provider: "mock", ProviderMarketID: "mk-1",
	})
	require.NoError(t, err)
	_, err = store.UpsertOutcome(ctx, oddsfeed.Outcome{
		ID: "outcome-1", MarketID: "market-1", Name: "Home", Odds: "2.10", Status: "active",
		Provider: "mock", ProviderOutcomeID: "oc-1",
	})
	require.NoError(t, err)

	listener := bufconn.Listen(1024 * 1024)
	grpcServer := grpc.NewServer()
	pb.RegisterOddsFeedServiceServer(grpcServer, oddsfeed.NewGRPCServer(svc))
	go func() {
		if err := grpcServer.Serve(listener); err != nil {
			t.Logf("grpc server error: %v", err)
		}
	}()
	return store, listener, grpcServer
}

func newOddsFeedGatewayServer(t *testing.T, listener *bufconn.Listener) *Server {
	t.Helper()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
		return listener.Dial()
	}), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })

	oddsClient := &OddsFeedClient{conn: pb.NewOddsFeedServiceClient(conn)}
	return NewServer(logger, nil, oddsClient, nil, NewRateLimiter("memory", "", 100, 100), "", "", "", "", "", Limits{})
}

func TestGatewayListSports(t *testing.T) {
	_, listener, grpcServer := newOddsFeedServer(t)
	defer grpcServer.Stop()

	srv := newOddsFeedGatewayServer(t, listener)
	req := httptest.NewRequest(http.MethodGet, "/api/sports", nil)
	w := httptest.NewRecorder()

	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Soccer")
	assert.Contains(t, w.Body.String(), "sport-1")
}

func TestGatewayListLeagues(t *testing.T) {
	_, listener, grpcServer := newOddsFeedServer(t)
	defer grpcServer.Stop()

	srv := newOddsFeedGatewayServer(t, listener)
	req := httptest.NewRequest(http.MethodGet, "/api/sports/sport-1/leagues", nil)
	w := httptest.NewRecorder()

	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Mock League")
	assert.Contains(t, w.Body.String(), "league-1")
}

func TestGatewayListEvents(t *testing.T) {
	_, listener, grpcServer := newOddsFeedServer(t)
	defer grpcServer.Stop()

	srv := newOddsFeedGatewayServer(t, listener)
	req := httptest.NewRequest(http.MethodGet, "/api/events?sport_id=sport-1&league_id=league-1", nil)
	w := httptest.NewRecorder()

	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Mock FC")
	assert.Contains(t, w.Body.String(), "Test United")
	assert.Contains(t, w.Body.String(), "event-1")
}

func TestGatewayGetEvent(t *testing.T) {
	_, listener, grpcServer := newOddsFeedServer(t)
	defer grpcServer.Stop()

	srv := newOddsFeedGatewayServer(t, listener)
	req := httptest.NewRequest(http.MethodGet, "/api/events/event-1", nil)
	w := httptest.NewRecorder()

	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Mock FC")
	assert.Contains(t, w.Body.String(), "event-1")
}

func TestGatewayListMarkets(t *testing.T) {
	_, listener, grpcServer := newOddsFeedServer(t)
	defer grpcServer.Stop()

	srv := newOddsFeedGatewayServer(t, listener)
	req := httptest.NewRequest(http.MethodGet, "/api/events/event-1/markets", nil)
	w := httptest.NewRecorder()

	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Match Result")
	assert.Contains(t, w.Body.String(), "market-1")
}

func TestGatewayListOutcomes(t *testing.T) {
	_, listener, grpcServer := newOddsFeedServer(t)
	defer grpcServer.Stop()

	srv := newOddsFeedGatewayServer(t, listener)
	req := httptest.NewRequest(http.MethodGet, "/api/markets/market-1/outcomes", nil)
	w := httptest.NewRecorder()

	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Home")
	assert.Contains(t, w.Body.String(), "2.10")
	assert.Contains(t, w.Body.String(), "outcome-1")
}

func TestGatewayListLiveEvents(t *testing.T) {
	_, listener, grpcServer := newOddsFeedServer(t)
	defer grpcServer.Stop()

	srv := newOddsFeedGatewayServer(t, listener)
	req := httptest.NewRequest(http.MethodGet, "/api/live/events", nil)
	w := httptest.NewRecorder()

	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, "events")
	var resp struct {
		Events []json.RawMessage `json:"events"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Empty(t, resp.Events)
}

func TestGatewayListLiveEventsWithLiveEvent(t *testing.T) {
	store, listener, grpcServer := newOddsFeedServer(t)
	defer grpcServer.Stop()

	ctx := context.Background()
	_, err := store.UpsertEvent(ctx, oddsfeed.Event{
		ID: "event-live-1", LeagueID: "league-1", SportID: "sport-1",
		HomeParticipant: "Live FC", AwayParticipant: "Live United",
		Status: "live", StartsAt: time.Now(),
		Provider: "mock", ProviderEventID: "ev-live-1",
	})
	require.NoError(t, err)

	srv := newOddsFeedGatewayServer(t, listener)
	req := httptest.NewRequest(http.MethodGet, "/api/live/events", nil)
	w := httptest.NewRecorder()

	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Live FC")
	assert.Contains(t, w.Body.String(), "event-live-1")
}

func TestGatewayGetEventNotFound(t *testing.T) {
	_, listener, grpcServer := newOddsFeedServer(t)
	defer grpcServer.Stop()

	srv := newOddsFeedGatewayServer(t, listener)
	req := httptest.NewRequest(http.MethodGet, "/api/events/nonexistent-id", nil)
	w := httptest.NewRecorder()

	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "not found")
}

func TestGatewayListEventsPagination(t *testing.T) {
	store, listener, grpcServer := newOddsFeedServer(t)
	defer grpcServer.Stop()

	ctx := context.Background()
	_, err := store.UpsertEvent(ctx, oddsfeed.Event{
		ID: "event-2", LeagueID: "league-1", SportID: "sport-1",
		HomeParticipant: "Second FC", AwayParticipant: "Other United",
		Status: "upcoming", StartsAt: time.Now().Add(3 * time.Hour),
		Provider: "mock", ProviderEventID: "ev-2",
	})
	require.NoError(t, err)

	srv := newOddsFeedGatewayServer(t, listener)
	req := httptest.NewRequest(http.MethodGet, "/api/events?page=1&page_size=1", nil)
	w := httptest.NewRecorder()

	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp struct {
		Events []json.RawMessage `json:"events"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Len(t, resp.Events, 1)
}
