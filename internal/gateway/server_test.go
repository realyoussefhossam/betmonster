package gateway

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
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

	"github.com/lestrrat-go/jwx/v3/jwa"
	"github.com/lestrrat-go/jwx/v3/jwk"
	"github.com/lestrrat-go/jwx/v3/jwt"

	"github.com/realyoussefhossam/betmonster/internal/auth"
	oddsfeed "github.com/realyoussefhossam/betmonster/internal/oddsfeed"
	pb "github.com/realyoussefhossam/betmonster/internal/proto"
	"github.com/realyoussefhossam/betmonster/internal/sportsbook"
	wallet "github.com/realyoussefhossam/betmonster/internal/wallet"
	"github.com/realyoussefhossam/betmonster/internal/wallet/rates"
	"github.com/realyoussefhossam/betmonster/internal/wallet/xcash"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/encoding/protojson"
)

func TestHealthHandler(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	srv := NewServer(logger, nil, nil, nil, nil, NewRateLimiter("memory", "", 100, 100), "", "", "USDT", "anvil", "", Limits{})
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "healthy")
}

func TestSupportedOptions(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	srv := NewServer(logger, nil, nil, nil, nil, NewRateLimiter("memory", "", 100, 100), "", "", "USDT,USDC", "anvil,base", "", Limits{})
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
	srv := NewServer(logger, nil, nil, nil, nil, NewRateLimiter("memory", "", 100, 100), "", "", "USDT", "anvil", "", Limits{})
	req := httptest.NewRequest(http.MethodGet, "/api/wallet/balance?currency=BTC", nil)
	req = req.WithContext(context.WithValue(req.Context(), UserContextKey, auth.User{ID: "user-1"}))
	w := httptest.NewRecorder()

	srv.handleBalance(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "unsupported currency")
}

func TestHandleDepositAddressUnsupportedPair(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	srv := NewServer(logger, nil, nil, nil, nil, NewRateLimiter("memory", "", 100, 100), "", "", "USDT", "anvil", "USDT:anvil", Limits{})
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
	srv := NewServer(logger, walletClient, nil, nil, nil, NewRateLimiter("memory", "", 100, 100), "", "", "USDT", "anvil", "USDT:anvil", Limits{})

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
	srv := NewServer(logger, walletClient, nil, nil, nil, NewRateLimiter("memory", "", 100, 100), "", "", "USDT", "anvil", "USDT:anvil", Limits{})

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
	srv := NewServer(logger, walletClient, nil, nil, nil, NewRateLimiter("memory", "", 100, 100), "", "", "USDT", "anvil", "USDT:anvil", Limits{})

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
	return NewServer(logger, nil, oddsClient, nil, nil, NewRateLimiter("memory", "", 100, 100), "", "", "", "", "", Limits{})
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

func TestGatewayMarkets(t *testing.T) {
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

func TestGatewayOutcomes(t *testing.T) {
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

func newTestJWKSClient(t *testing.T) (*auth.JWKSClient, func(userID string) string) {
	t.Helper()
	rawKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	jwkKey, err := jwk.Import(rawKey)
	require.NoError(t, err)
	require.NoError(t, jwkKey.Set(jwk.KeyIDKey, "test-key"))
	require.NoError(t, jwkKey.Set(jwk.AlgorithmKey, jwa.RS256()))

	pubkey, err := jwk.PublicKeyOf(jwkKey)
	require.NoError(t, err)
	require.NoError(t, pubkey.Set(jwk.KeyIDKey, "test-key"))
	require.NoError(t, pubkey.Set(jwk.AlgorithmKey, jwa.RS256()))

	set := jwk.NewSet()
	require.NoError(t, set.AddKey(pubkey))

	jwksBytes, err := json.Marshal(set)
	require.NoError(t, err)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(jwksBytes)
	}))
	t.Cleanup(server.Close)

	client, err := auth.NewJWKSClient(context.Background(), server.URL)
	require.NoError(t, err)

	tokenFn := func(userID string) string {
		tok, err := jwt.NewBuilder().Subject(userID).Build()
		require.NoError(t, err)
		signed, err := jwt.Sign(tok, jwt.WithKey(jwa.RS256(), jwkKey))
		require.NoError(t, err)
		return "Bearer " + string(signed)
	}

	return client, tokenFn
}

func newWalletServer(t *testing.T) (*WalletClient, wallet.Store, *grpc.Server) {
	t.Helper()
	store := wallet.NewInMemoryStore()
	validator := xcash.NewWebhookValidator("hmac-key")
	svc := wallet.NewService(store, nil, validator, []string{"USDT:anvil"})
	agg := rates.NewAggregator(rates.NewCache(time.Hour), rates.NewForexChain())
	grpcServer := grpc.NewServer(grpc.UnaryInterceptor(wallet.AuthInterceptor))
	pb.RegisterWalletServiceServer(grpcServer, wallet.NewGRPCServer(svc, agg))

	listener := bufconn.Listen(1024 * 1024)
	go func() {
		if err := grpcServer.Serve(listener); err != nil {
			t.Logf("grpc server error: %v", err)
		}
	}()
	t.Cleanup(grpcServer.Stop)

	conn, err := grpc.Dial("bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
		return listener.Dial()
	}), grpc.WithInsecure())
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })

	return &WalletClient{conn: pb.NewWalletServiceClient(conn)}, store, grpcServer
}

func TestGatewayEndpoints(t *testing.T) {
	walletClient, store, _ := newWalletServer(t)
	jwksClient, token := newTestJWKSClient(t)

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	srv := NewServer(logger, walletClient, nil, nil, jwksClient, NewRateLimiter("memory", "", 100, 100), "admin-1", "", "USDT", "anvil", "USDT:anvil", Limits{})

	ctx := context.Background()
	_, err := store.CreditWallet(ctx, "user-1", "USDT", "100.00", "seed", nil)
	require.NoError(t, err)
	_, err = store.CreateDepositAddress(ctx, &wallet.DepositAddress{
		UserID: "user-1", Currency: "USDT", Chain: "anvil", Address: "0xDepositAddress", Status: "active",
	})
	require.NoError(t, err)

	t.Run("wallet supported", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/wallet/supported", nil)
		w := httptest.NewRecorder()
		srv.Router().ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		var resp struct {
			Currencies []string `json:"currencies"`
			Chains     []string `json:"chains"`
			Pairs      []string `json:"pairs"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.Contains(t, resp.Currencies, "USDT")
		assert.Contains(t, resp.Chains, "anvil")
		assert.Contains(t, resp.Pairs, "USDT:anvil")
	})

	t.Run("wallet rates", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/wallet/rates", nil)
		w := httptest.NewRecorder()
		srv.Router().ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		var resp struct {
			FiatCurrency string            `json:"fiat_currency"`
			Rates        map[string]string `json:"rates"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.Equal(t, "USD", resp.FiatCurrency)
		assert.NotEmpty(t, resp.Rates)
	})

	t.Run("wallet balance", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/wallet/balance?currency=USDT", nil)
		req.Header.Set("Authorization", token("user-1"))
		w := httptest.NewRecorder()
		srv.Router().ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		var resp struct {
			Currency     string `json:"currency"`
			Balance      string `json:"balance"`
			FiatCurrency string `json:"fiatCurrency"`
			FiatValue    string `json:"fiatValue"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.Equal(t, "USDT", resp.Currency)
		assert.Equal(t, "100", resp.Balance)
	})

	t.Run("wallet transactions", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/wallet/transactions", nil)
		req.Header.Set("Authorization", token("user-1"))
		w := httptest.NewRecorder()
		srv.Router().ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		var resp struct {
			Transactions []json.RawMessage `json:"transactions"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.Len(t, resp.Transactions, 1)
	})

	t.Run("wallet deposit address", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/wallet/deposit-address?currency=USDT&chain=anvil", nil)
		req.Header.Set("Authorization", token("user-1"))
		w := httptest.NewRecorder()
		srv.Router().ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		var resp struct {
			Address  string `json:"address"`
			Currency string `json:"currency"`
			Chain    string `json:"chain"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.Equal(t, "0xDepositAddress", resp.Address)
		assert.Equal(t, "USDT", resp.Currency)
		assert.Equal(t, "anvil", resp.Chain)
	})

	t.Run("wallet withdraw", func(t *testing.T) {
		body := `{"currency":"USDT","amount":"10","destinationAddress":"0xDest","chain":"anvil"}`
		req := httptest.NewRequest(http.MethodPost, "/api/wallet/withdraw", strings.NewReader(body))
		req.Header.Set("Authorization", token("user-1"))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		srv.Router().ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		var resp struct {
			WithdrawalID string `json:"withdrawalId"`
			Status       string `json:"status"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.NotEmpty(t, resp.WithdrawalID)
		assert.Equal(t, "pending", resp.Status)
	})
}

func TestGatewayAdmin(t *testing.T) {
	walletClient, store, _ := newWalletServer(t)
	jwksClient, token := newTestJWKSClient(t)

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	srv := NewServer(logger, walletClient, nil, nil, jwksClient, NewRateLimiter("memory", "", 100, 100), "admin-1", "", "USDT", "anvil", "USDT:anvil", Limits{})

	ctx := context.Background()
	_, err := store.CreditWallet(ctx, "user-1", "USDT", "100.00", "seed", nil)
	require.NoError(t, err)
	wd, err := store.RequestWithdrawal(ctx, "user-1", "USDT", "10.00", "0xDest", "anvil")
	require.NoError(t, err)

	t.Run("list withdrawals unauthenticated", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/admin/withdrawals", nil)
		w := httptest.NewRecorder()
		srv.Router().ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("list withdrawals non-admin", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/admin/withdrawals", nil)
		req.Header.Set("Authorization", token("user-1"))
		w := httptest.NewRecorder()
		srv.Router().ServeHTTP(w, req)
		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("list withdrawals admin", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/admin/withdrawals", nil)
		req.Header.Set("Authorization", token("admin-1"))
		w := httptest.NewRecorder()
		srv.Router().ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		var resp struct {
			Withdrawals []json.RawMessage `json:"withdrawals"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.Len(t, resp.Withdrawals, 1)
	})

	t.Run("review withdrawal unauthenticated", func(t *testing.T) {
		body := `{"withdrawalId":"` + wd.ID + `","action":"approve","txHash":"0xabc"}`
		req := httptest.NewRequest(http.MethodPost, "/api/admin/withdrawals/review", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		srv.Router().ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("review withdrawal non-admin", func(t *testing.T) {
		body := `{"withdrawalId":"` + wd.ID + `","action":"approve","txHash":"0xabc"}`
		req := httptest.NewRequest(http.MethodPost, "/api/admin/withdrawals/review", strings.NewReader(body))
		req.Header.Set("Authorization", token("user-1"))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		srv.Router().ServeHTTP(w, req)
		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("review withdrawal admin", func(t *testing.T) {
		body := `{"withdrawalId":"` + wd.ID + `","action":"approve","txHash":"0xabc"}`
		req := httptest.NewRequest(http.MethodPost, "/api/admin/withdrawals/review", strings.NewReader(body))
		req.Header.Set("Authorization", token("admin-1"))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		srv.Router().ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		var resp struct {
			Status string `json:"status"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.Equal(t, "approved", resp.Status)
	})
}

func TestGatewayWebhookInvalidSignature(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

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
	srv := NewServer(logger, walletClient, nil, nil, nil, NewRateLimiter("memory", "", 100, 100), "", "", "USDT", "anvil", "USDT:anvil", Limits{})

	body := `{"type":"deposit","data":{"sys_no":"DXC1","uid":"u1","amount":"10","crypto":"USDT","chain":"anvil","confirmed":true,"hash":"0xabc","block":1,"risk_level":null,"risk_score":null}}`
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	req := httptest.NewRequest(http.MethodPost, "/webhooks/xcash/deposit", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("XC-Nonce", "nonce")
	req.Header.Set("XC-Timestamp", timestamp)
	req.Header.Set("XC-Signature", "invalid-signature")
	w := httptest.NewRecorder()

	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "error")
}

type gatewayMockWallet struct{}

func (m *gatewayMockWallet) DebitWallet(ctx context.Context, userID, currency, amount, referenceID string, metadata map[string]any) (string, error) {
	return "", nil
}

func (m *gatewayMockWallet) CreditWallet(ctx context.Context, userID, currency, amount, referenceID string, metadata map[string]any) (string, error) {
	return "", nil
}

type gatewayMockOddsFeed struct {
	eventID   string
	marketID  string
	outcomeID string
	odds      string
}

func (m *gatewayMockOddsFeed) GetEvent(ctx context.Context, id string) (*pb.Event, error) {
	if id == m.eventID {
		return &pb.Event{Id: m.eventID, Status: "upcoming"}, nil
	}
	return nil, nil
}

func (m *gatewayMockOddsFeed) ListMarkets(ctx context.Context, eventID, status string, page, pageSize int) ([]*pb.Market, error) {
	if eventID == m.eventID {
		return []*pb.Market{{Id: m.marketID, EventId: m.eventID, Status: "active"}}, nil
	}
	return nil, nil
}

func (m *gatewayMockOddsFeed) ListOutcomes(ctx context.Context, marketID, status string, page, pageSize int) ([]*pb.Outcome, error) {
	if marketID == m.marketID {
		return []*pb.Outcome{{Id: m.outcomeID, MarketId: m.marketID, Odds: m.odds, Status: "active"}}, nil
	}
	return nil, nil
}

func (m *gatewayMockOddsFeed) ListEvents(ctx context.Context, sportID, leagueID, status string, page, pageSize int) ([]*pb.Event, error) {
	return nil, nil
}

func newSportsbookServerForGateway(t *testing.T) (*grpc.Server, *bufconn.Listener) {
	t.Helper()
	store := sportsbook.NewInMemoryStore()
	wallet := &gatewayMockWallet{}
	oddsfeed := &gatewayMockOddsFeed{eventID: "event-1", marketID: "market-1", outcomeID: "outcome-1", odds: "2.10"}
	svc := sportsbook.NewService(store, wallet, oddsfeed)
	grpcServer := grpc.NewServer(grpc.UnaryInterceptor(sportsbook.AuthInterceptor))
	pb.RegisterSportsbookServiceServer(grpcServer, sportsbook.NewGRPCServer(svc))

	listener := bufconn.Listen(1024 * 1024)
	go func() {
		if err := grpcServer.Serve(listener); err != nil {
			t.Logf("sportsbook grpc server error: %v", err)
		}
	}()
	t.Cleanup(grpcServer.Stop)
	return grpcServer, listener
}

func newSportsbookGatewayServer(t *testing.T, listener *bufconn.Listener) (*Server, func()) {
	t.Helper()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
		return listener.Dial()
	}), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)

	oddsClient := &SportsbookClient{conn: pb.NewSportsbookServiceClient(conn)}
	srv := NewServer(logger, nil, nil, oddsClient, nil, NewRateLimiter("memory", "", 100, 100), "", "", "", "", "", Limits{})
	return srv, func() { conn.Close() }
}

func TestGatewayPlaceBet(t *testing.T) {
	_, listener := newSportsbookServerForGateway(t)
	srv, cleanup := newSportsbookGatewayServer(t, listener)
	defer cleanup()
	jwksClient, token := newTestJWKSClient(t)
	srv.jwksClient = jwksClient

	body := `{"eventId":"event-1","marketId":"market-1","outcomeId":"outcome-1","stake":"10.00","currency":"USDT","referenceId":"ref-gw-1"}`
	req := httptest.NewRequest(http.MethodPost, "/api/bets", strings.NewReader(body))
	req.Header.Set("Authorization", token("user-1"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	var resp pb.PlaceBetResponse
	require.NoError(t, protojson.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "user-1", resp.Bet.UserId)
	assert.Equal(t, "21", resp.Bet.PotentialPayout)
}

func TestGatewayListBets(t *testing.T) {
	_, listener := newSportsbookServerForGateway(t)
	srv, cleanup := newSportsbookGatewayServer(t, listener)
	defer cleanup()
	jwksClient, token := newTestJWKSClient(t)
	srv.jwksClient = jwksClient

	ctx := context.Background()
	_, err := srv.sportsbook.PlaceBet(ctx, "user-1", "event-1", "market-1", "outcome-1", "10.00", "USDT", "ref-gw-2")
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/bets", nil)
	req.Header.Set("Authorization", token("user-1"))
	w := httptest.NewRecorder()

	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp pb.ListBetsResponse
	require.NoError(t, protojson.Unmarshal(w.Body.Bytes(), &resp))
	assert.Len(t, resp.Bets, 1)
	assert.Equal(t, "user-1", resp.Bets[0].UserId)
}
