package wallet

import (
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"

	pb "github.com/realyoussefhossam/betmonster/internal/proto"
)

func TestGRPCServerGetBalance(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryStore()
	_, err := store.CreditWallet(ctx, "user-1", "USDT", "100.00", "dx-1", nil)
	assert.NoError(t, err)

	svc := NewService(store, nil, nil, []string{"USDT:anvil"})
	server := NewGRPCServer(svc)

	listener := bufconn.Listen(1024)
	grpcServer := grpc.NewServer()
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

func TestGRPCServerListTransactionsIncludesCreatedAt(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryStore()
	_, err := store.CreditWallet(ctx, "user-1", "USDT", "100.00", "dx-1", nil)
	assert.NoError(t, err)

	svc := NewService(store, nil, nil, []string{"USDT:anvil"})
	server := NewGRPCServer(svc)

	listener := bufconn.Listen(1024)
	grpcServer := grpc.NewServer()
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
