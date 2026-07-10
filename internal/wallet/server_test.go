package wallet

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	pb "github.com/realyoussefhossam/betmonster/internal/proto"
	"github.com/realyoussefhossam/betmonster/internal/shared/grpcmeta"
	"github.com/realyoussefhossam/betmonster/internal/wallet/rates"
	"github.com/realyoussefhossam/betmonster/internal/wallet/xcash"
)

func TestGRPCServerGetBalance(t *testing.T) {
	ctx := metadata.AppendToOutgoingContext(context.Background(), grpcmeta.UserIDHeader, "user-1")
	store := NewInMemoryStore()
	_, err := store.CreditWallet(ctx, "user-1", "USDT", "100.00", "dx-1", nil)
	assert.NoError(t, err)

	svc := NewService(store, nil, nil, []string{"USDT:anvil"})
	server := NewGRPCServer(svc, nil)

	listener := bufconn.Listen(1024)
	grpcServer := grpc.NewServer(grpc.UnaryInterceptor(AuthInterceptor))
	pb.RegisterWalletServiceServer(grpcServer, server)

	go func() {
		if err := grpcServer.Serve(listener); err != nil {
			t.Logf("grpc server error: %v", err)
		}
	}()
	defer grpcServer.Stop()

	conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
		return listener.Dial()
	}), grpc.WithInsecure())
	assert.NoError(t, err)
	defer conn.Close()

	client := pb.NewWalletServiceClient(conn)
	resp, err := client.GetBalance(ctx, &pb.GetBalanceRequest{UserId: "user-1", Currency: "USDT"})
	assert.NoError(t, err)
	assert.Equal(t, "USDT", resp.Currency)
	assert.Equal(t, "100", resp.Balance)
}

func TestGRPCServerGetBalanceWithFiatValue(t *testing.T) {
	ctx := metadata.AppendToOutgoingContext(context.Background(), grpcmeta.UserIDHeader, "user-1")
	store := NewInMemoryStore()
	_, err := store.CreditWallet(ctx, "user-1", "USDT", "100.00", "dx-1", nil)
	assert.NoError(t, err)

	svc := NewService(store, nil, nil, []string{"USDT:anvil"})
	cache := rates.NewCache(30 * time.Second)
	agg := rates.NewAggregator(cache, rates.NewForexChain())
	server := NewGRPCServer(svc, agg)

	listener := bufconn.Listen(1024)
	grpcServer := grpc.NewServer(grpc.UnaryInterceptor(AuthInterceptor))
	pb.RegisterWalletServiceServer(grpcServer, server)

	go func() {
		if err := grpcServer.Serve(listener); err != nil {
			t.Logf("grpc server error: %v", err)
		}
	}()
	defer grpcServer.Stop()

	conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
		return listener.Dial()
	}), grpc.WithInsecure())
	assert.NoError(t, err)
	defer conn.Close()

	client := pb.NewWalletServiceClient(conn)
	resp, err := client.GetBalance(ctx, &pb.GetBalanceRequest{UserId: "user-1", Currency: "USDT"})
	assert.NoError(t, err)
	assert.Equal(t, "USDT", resp.Currency)
	assert.Equal(t, "USD", resp.FiatCurrency)
	assert.Equal(t, "100.00", resp.FiatValue)
}

func TestGRPCServerListTransactionsIncludesCreatedAt(t *testing.T) {
	ctx := metadata.AppendToOutgoingContext(context.Background(), grpcmeta.UserIDHeader, "user-1")
	store := NewInMemoryStore()
	_, err := store.CreditWallet(ctx, "user-1", "USDT", "100.00", "dx-1", nil)
	assert.NoError(t, err)

	svc := NewService(store, nil, nil, []string{"USDT:anvil"})
	server := NewGRPCServer(svc, nil)

	listener := bufconn.Listen(1024)
	grpcServer := grpc.NewServer(grpc.UnaryInterceptor(AuthInterceptor))
	pb.RegisterWalletServiceServer(grpcServer, server)

	go func() {
		if err := grpcServer.Serve(listener); err != nil {
			t.Logf("grpc server error: %v", err)
		}
	}()
	defer grpcServer.Stop()

	conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
		return listener.Dial()
	}), grpc.WithInsecure())
	assert.NoError(t, err)
	defer conn.Close()

	client := pb.NewWalletServiceClient(conn)
	resp, err := client.ListTransactions(ctx, &pb.ListTransactionsRequest{UserId: "user-1"})
	assert.NoError(t, err)
	assert.Len(t, resp.Transactions, 1)
	assert.NotEmpty(t, resp.Transactions[0].CreatedAt, "created_at should be populated")
}

func TestGRPCServerGetBalance_EUR(t *testing.T) {
	ctx := metadata.AppendToOutgoingContext(context.Background(), grpcmeta.UserIDHeader, "user-1")
	store := NewInMemoryStore()
	_, err := store.CreditWallet(ctx, "user-1", "USDT", "100.00", "dx-1", nil)
	require.NoError(t, err)

	t.Setenv("MANUAL_USD_RATES", `{"EUR":"0.92"}`)
	r := rates.NewAggregator(rates.NewCache(30*time.Second), rates.NewForexChain(), rates.NewBinance())
	srv := NewGRPCServer(NewService(store, nil, nil, []string{"USDT:anvil"}), r)

	resp, err := srv.GetBalance(ctx, &pb.GetBalanceRequest{UserId: "user-1", Currency: "USDT", FiatCurrency: "EUR"})
	require.NoError(t, err)
	assert.Equal(t, "EUR", resp.FiatCurrency)
	assert.Equal(t, "92.00", resp.FiatValue)
}

func TestAuthInterceptorMissingMetadata(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryStore()
	svc := NewService(store, nil, nil, []string{"USDT:anvil"})
	server := NewGRPCServer(svc, nil)

	listener := bufconn.Listen(1024)
	grpcServer := grpc.NewServer(grpc.UnaryInterceptor(AuthInterceptor))
	pb.RegisterWalletServiceServer(grpcServer, server)

	go func() {
		if err := grpcServer.Serve(listener); err != nil {
			t.Logf("grpc server error: %v", err)
		}
	}()
	defer grpcServer.Stop()

	conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
		return listener.Dial()
	}), grpc.WithInsecure())
	require.NoError(t, err)
	defer conn.Close()

	client := pb.NewWalletServiceClient(conn)
	_, err = client.GetBalance(ctx, &pb.GetBalanceRequest{UserId: "user-1", Currency: "USDT"})
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Unauthenticated, st.Code())
	assert.Contains(t, st.Message(), "missing caller")
}

func TestAuthInterceptorEmptyUserID(t *testing.T) {
	ctx := metadata.AppendToOutgoingContext(context.Background(), grpcmeta.UserIDHeader, "")
	store := NewInMemoryStore()
	svc := NewService(store, nil, nil, []string{"USDT:anvil"})
	server := NewGRPCServer(svc, nil)

	listener := bufconn.Listen(1024)
	grpcServer := grpc.NewServer(grpc.UnaryInterceptor(AuthInterceptor))
	pb.RegisterWalletServiceServer(grpcServer, server)

	go func() {
		if err := grpcServer.Serve(listener); err != nil {
			t.Logf("grpc server error: %v", err)
		}
	}()
	defer grpcServer.Stop()

	conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
		return listener.Dial()
	}), grpc.WithInsecure())
	require.NoError(t, err)
	defer conn.Close()

	client := pb.NewWalletServiceClient(conn)
	_, err = client.GetBalance(ctx, &pb.GetBalanceRequest{UserId: "user-1", Currency: "USDT"})
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Unauthenticated, st.Code())
	assert.Contains(t, st.Message(), "missing caller")
}

func TestAuthInterceptorUserIDMismatch(t *testing.T) {
	ctx := metadata.AppendToOutgoingContext(context.Background(), grpcmeta.UserIDHeader, "user-1")
	store := NewInMemoryStore()
	svc := NewService(store, nil, nil, []string{"USDT:anvil"})
	server := NewGRPCServer(svc, nil)

	listener := bufconn.Listen(1024)
	grpcServer := grpc.NewServer(grpc.UnaryInterceptor(AuthInterceptor))
	pb.RegisterWalletServiceServer(grpcServer, server)

	go func() {
		if err := grpcServer.Serve(listener); err != nil {
			t.Logf("grpc server error: %v", err)
		}
	}()
	defer grpcServer.Stop()

	conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
		return listener.Dial()
	}), grpc.WithInsecure())
	require.NoError(t, err)
	defer conn.Close()

	client := pb.NewWalletServiceClient(conn)
	_, err = client.GetBalance(ctx, &pb.GetBalanceRequest{UserId: "user-2", Currency: "USDT"})
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.PermissionDenied, st.Code())
	assert.Contains(t, st.Message(), "caller user id does not match request user id")
}

func TestAuthInterceptorAllowsSystemCalls(t *testing.T) {
	ctx := metadata.AppendToOutgoingContext(context.Background(), grpcmeta.UserIDHeader, "gateway")
	store := NewInMemoryStore()
	svc := NewService(store, nil, nil, []string{"USDT:anvil"})
	server := NewGRPCServer(svc, nil)

	listener := bufconn.Listen(1024)
	grpcServer := grpc.NewServer(grpc.UnaryInterceptor(AuthInterceptor))
	pb.RegisterWalletServiceServer(grpcServer, server)

	go func() {
		if err := grpcServer.Serve(listener); err != nil {
			t.Logf("grpc server error: %v", err)
		}
	}()
	defer grpcServer.Stop()

	conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
		return listener.Dial()
	}), grpc.WithInsecure())
	require.NoError(t, err)
	defer conn.Close()

	client := pb.NewWalletServiceClient(conn)
	resp, err := client.GetRates(ctx, &pb.GetRatesRequest{})
	require.NoError(t, err)
	assert.Equal(t, "USD", resp.FiatCurrency)
}

func TestAuthInterceptorAdminMethodRequiresAdminMetadata(t *testing.T) {
	ctx := metadata.AppendToOutgoingContext(context.Background(), grpcmeta.UserIDHeader, "gateway")
	store := NewInMemoryStore()
	svc := NewService(store, nil, nil, []string{"USDT:anvil"})
	server := NewGRPCServer(svc, nil)

	listener := bufconn.Listen(1024)
	grpcServer := grpc.NewServer(grpc.UnaryInterceptor(AuthInterceptor))
	pb.RegisterWalletServiceServer(grpcServer, server)

	go func() {
		if err := grpcServer.Serve(listener); err != nil {
			t.Logf("grpc server error: %v", err)
		}
	}()
	defer grpcServer.Stop()

	conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
		return listener.Dial()
	}), grpc.WithInsecure())
	require.NoError(t, err)
	defer conn.Close()

	client := pb.NewWalletServiceClient(conn)
	_, err = client.ListPendingWithdrawals(ctx, &pb.ListPendingWithdrawalsRequest{})
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.PermissionDenied, st.Code())
	assert.Contains(t, st.Message(), "admin metadata required")
}

func TestAuthInterceptorAdminMethodSucceedsWithAdminMetadata(t *testing.T) {
	ctx := metadata.AppendToOutgoingContext(context.Background(), grpcmeta.UserIDHeader, "gateway", grpcmeta.IsAdminHeader, "true")
	store := NewInMemoryStore()
	svc := NewService(store, nil, nil, []string{"USDT:anvil"})
	server := NewGRPCServer(svc, nil)

	listener := bufconn.Listen(1024)
	grpcServer := grpc.NewServer(grpc.UnaryInterceptor(AuthInterceptor))
	pb.RegisterWalletServiceServer(grpcServer, server)

	go func() {
		if err := grpcServer.Serve(listener); err != nil {
			t.Logf("grpc server error: %v", err)
		}
	}()
	defer grpcServer.Stop()

	conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
		return listener.Dial()
	}), grpc.WithInsecure())
	require.NoError(t, err)
	defer conn.Close()

	client := pb.NewWalletServiceClient(conn)
	resp, err := client.ListPendingWithdrawals(ctx, &pb.ListPendingWithdrawalsRequest{})
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Empty(t, resp.Withdrawals)
}

func TestAuthInterceptorSystemCallsWithoutUserMatch(t *testing.T) {
	key := "test-webhook-key"
	validator := xcash.NewWebhookValidator(key)

	bodyMap := map[string]any{
		"type": "deposit",
		"data": map[string]any{
			"sys_no":     "sys-1",
			"uid":        "user-1",
			"chain":      "anvil",
			"block":      1,
			"hash":       "0xabc",
			"crypto":     "USDT",
			"amount":     "10.00",
			"confirmed":  false,
			"risk_level": "low",
			"risk_score": "0",
		},
	}
	body, err := json.Marshal(bodyMap)
	require.NoError(t, err)

	nonce := hex.EncodeToString([]byte("nonce-1"))
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	signature := xcash.Sign(nonce+timestamp+string(body), key)
	headers := map[string]string{
		"XC-Nonce":     nonce,
		"XC-Timestamp": timestamp,
		"XC-Signature": signature,
	}

	ctx := metadata.AppendToOutgoingContext(context.Background(), grpcmeta.UserIDHeader, "gateway")
	store := NewInMemoryStore()
	svc := NewService(store, nil, validator, []string{"USDT:anvil"})
	server := NewGRPCServer(svc, nil)

	listener := bufconn.Listen(1024)
	grpcServer := grpc.NewServer(grpc.UnaryInterceptor(AuthInterceptor))
	pb.RegisterWalletServiceServer(grpcServer, server)

	go func() {
		if err := grpcServer.Serve(listener); err != nil {
			t.Logf("grpc server error: %v", err)
		}
	}()
	defer grpcServer.Stop()

	conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
		return listener.Dial()
	}), grpc.WithInsecure())
	require.NoError(t, err)
	defer conn.Close()

	client := pb.NewWalletServiceClient(conn)
	resp, err := client.ProcessDepositWebhook(ctx, &pb.ProcessDepositWebhookRequest{Body: string(body), Headers: headers})
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func privilegedCtx() context.Context {
	return metadata.AppendToOutgoingContext(context.Background(),
		grpcmeta.UserIDHeader, serviceCallerID,
		grpcmeta.IsAdminHeader, "true",
	)
}

func userCtx(userID string) context.Context {
	return metadata.AppendToOutgoingContext(context.Background(), grpcmeta.UserIDHeader, userID)
}

func TestGRPCServerDebitWallet(t *testing.T) {
	ctx := privilegedCtx()
	store := NewInMemoryStore()
	_, err := store.CreditWallet(ctx, "user-1", "USDT", "100.00", "dx-1", nil)
	require.NoError(t, err)

	svc := NewService(store, nil, nil, []string{"USDT:anvil"})
	server := NewGRPCServer(svc, nil)

	listener := bufconn.Listen(1024)
	grpcServer := grpc.NewServer(grpc.UnaryInterceptor(AuthInterceptor))
	pb.RegisterWalletServiceServer(grpcServer, server)

	go func() {
		if err := grpcServer.Serve(listener); err != nil {
			t.Logf("grpc server error: %v", err)
		}
	}()
	defer grpcServer.Stop()

	conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
		return listener.Dial()
	}), grpc.WithInsecure())
	require.NoError(t, err)
	defer conn.Close()

	client := pb.NewWalletServiceClient(conn)
	resp, err := client.DebitWallet(ctx, &pb.DebitWalletRequest{
		UserId: "user-1", Currency: "USDT", Amount: "30.00", ReferenceId: "debit-1", Metadata: `{"reason":"stake"}`,
	})
	require.NoError(t, err)
	assert.Equal(t, "completed", resp.Status)
	assert.NotEmpty(t, resp.TransactionId)

	bal, err := client.GetBalance(userCtx("user-1"), &pb.GetBalanceRequest{UserId: "user-1", Currency: "USDT"})
	require.NoError(t, err)
	assert.Equal(t, "70", bal.Balance)
}

func TestGRPCServerCreditWallet(t *testing.T) {
	ctx := privilegedCtx()
	store := NewInMemoryStore()
	svc := NewService(store, nil, nil, []string{"USDT:anvil"})
	server := NewGRPCServer(svc, nil)

	listener := bufconn.Listen(1024)
	grpcServer := grpc.NewServer(grpc.UnaryInterceptor(AuthInterceptor))
	pb.RegisterWalletServiceServer(grpcServer, server)

	go func() {
		if err := grpcServer.Serve(listener); err != nil {
			t.Logf("grpc server error: %v", err)
		}
	}()
	defer grpcServer.Stop()

	conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
		return listener.Dial()
	}), grpc.WithInsecure())
	require.NoError(t, err)
	defer conn.Close()

	client := pb.NewWalletServiceClient(conn)
	resp, err := client.CreditWallet(ctx, &pb.CreditWalletRequest{
		UserId: "user-1", Currency: "USDT", Amount: "50.00", ReferenceId: "credit-1", Metadata: `{"reason":"winnings"}`,
	})
	require.NoError(t, err)
	assert.Equal(t, "completed", resp.Status)
	assert.NotEmpty(t, resp.TransactionId)

	bal, err := client.GetBalance(userCtx("user-1"), &pb.GetBalanceRequest{UserId: "user-1", Currency: "USDT"})
	require.NoError(t, err)
	assert.Equal(t, "50", bal.Balance)
}

func TestGRPCServerDebitWalletIdempotent(t *testing.T) {
	ctx := privilegedCtx()
	store := NewInMemoryStore()
	_, err := store.CreditWallet(ctx, "user-1", "USDT", "100.00", "dx-1", nil)
	require.NoError(t, err)

	svc := NewService(store, nil, nil, []string{"USDT:anvil"})
	server := NewGRPCServer(svc, nil)

	listener := bufconn.Listen(1024)
	grpcServer := grpc.NewServer(grpc.UnaryInterceptor(AuthInterceptor))
	pb.RegisterWalletServiceServer(grpcServer, server)

	go func() {
		if err := grpcServer.Serve(listener); err != nil {
			t.Logf("grpc server error: %v", err)
		}
	}()
	defer grpcServer.Stop()

	conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
		return listener.Dial()
	}), grpc.WithInsecure())
	require.NoError(t, err)
	defer conn.Close()

	client := pb.NewWalletServiceClient(conn)
	req := &pb.DebitWalletRequest{UserId: "user-1", Currency: "USDT", Amount: "30.00", ReferenceId: "debit-same", Metadata: ""}
	resp1, err := client.DebitWallet(ctx, req)
	require.NoError(t, err)

	resp2, err := client.DebitWallet(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, resp1.TransactionId, resp2.TransactionId, "duplicate reference_id should return the same transaction")

	bal, err := client.GetBalance(userCtx("user-1"), &pb.GetBalanceRequest{UserId: "user-1", Currency: "USDT"})
	require.NoError(t, err)
	assert.Equal(t, "70", bal.Balance, "balance should only be debited once")
}

func TestGRPCServerDebitWalletRejectsEndUserCaller(t *testing.T) {
	ctx := userCtx("user-1")
	store := NewInMemoryStore()
	svc := NewService(store, nil, nil, []string{"USDT:anvil"})
	server := NewGRPCServer(svc, nil)

	listener := bufconn.Listen(1024)
	grpcServer := grpc.NewServer(grpc.UnaryInterceptor(AuthInterceptor))
	pb.RegisterWalletServiceServer(grpcServer, server)

	go func() {
		if err := grpcServer.Serve(listener); err != nil {
			t.Logf("grpc server error: %v", err)
		}
	}()
	defer grpcServer.Stop()

	conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
		return listener.Dial()
	}), grpc.WithInsecure())
	require.NoError(t, err)
	defer conn.Close()

	client := pb.NewWalletServiceClient(conn)
	_, err = client.DebitWallet(ctx, &pb.DebitWalletRequest{
		UserId: "user-1", Currency: "USDT", Amount: "30.00", ReferenceId: "debit-user",
	})
	require.Error(t, err)
	assert.Equal(t, codes.PermissionDenied, status.Code(err), "end users must not be able to directly debit wallets")

	_, err = client.CreditWallet(ctx, &pb.CreditWalletRequest{
		UserId: "user-1", Currency: "USDT", Amount: "30.00", ReferenceId: "credit-user",
	})
	require.Error(t, err)
	assert.Equal(t, codes.PermissionDenied, status.Code(err), "end users must not be able to directly credit wallets")
}
