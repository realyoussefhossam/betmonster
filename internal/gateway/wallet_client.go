package gateway

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/realyoussefhossam/betmonster/internal/proto"
)

type WalletClient struct {
	conn pb.WalletServiceClient
}

func NewWalletClient(addr string) (*WalletClient, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, addr, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	if err != nil {
		return nil, fmt.Errorf("dial wallet service: %w", err)
	}
	return &WalletClient{conn: pb.NewWalletServiceClient(conn)}, nil
}

func (c *WalletClient) GetRates(ctx context.Context, fiat string) (*pb.GetRatesResponse, error) {
	return c.conn.GetRates(ctx, &pb.GetRatesRequest{FiatCurrency: fiat})
}

func (c *WalletClient) GetBalance(ctx context.Context, userID, currency, fiat string) (*pb.GetBalanceResponse, error) {
	return c.conn.GetBalance(ctx, &pb.GetBalanceRequest{UserId: userID, Currency: currency, FiatCurrency: fiat})
}

func (c *WalletClient) ListTransactions(ctx context.Context, userID, fiat string, page, pageSize int) (*pb.ListTransactionsResponse, error) {
	return c.conn.ListTransactions(ctx, &pb.ListTransactionsRequest{UserId: userID, FiatCurrency: fiat, Page: int32(page), PageSize: int32(pageSize)})
}

func (c *WalletClient) GetDepositAddress(ctx context.Context, userID, currency, chain string) (*pb.GetDepositAddressResponse, error) {
	return c.conn.GetDepositAddress(ctx, &pb.GetDepositAddressRequest{UserId: userID, Currency: currency, Chain: chain})
}

func (c *WalletClient) RequestWithdrawal(ctx context.Context, userID, currency, amount, destinationAddress, chain string) (*pb.RequestWithdrawalResponse, error) {
	return c.conn.RequestWithdrawal(ctx, &pb.RequestWithdrawalRequest{
		UserId: userID, Currency: currency, Amount: amount, DestinationAddress: destinationAddress, Chain: chain,
	})
}

func (c *WalletClient) ProcessDepositWebhook(ctx context.Context, body string, headers map[string]string) (*pb.ProcessDepositWebhookResponse, error) {
	return c.conn.ProcessDepositWebhook(ctx, &pb.ProcessDepositWebhookRequest{Body: body, Headers: headers})
}

func (c *WalletClient) ListPendingWithdrawals(ctx context.Context, page, pageSize int) (*pb.ListPendingWithdrawalsResponse, error) {
	return c.conn.ListPendingWithdrawals(ctx, &pb.ListPendingWithdrawalsRequest{Page: int32(page), PageSize: int32(pageSize)})
}

func (c *WalletClient) ReviewWithdrawal(ctx context.Context, id, action, txHash, reviewedBy string) (*pb.ReviewWithdrawalResponse, error) {
	return c.conn.ReviewWithdrawal(ctx, &pb.ReviewWithdrawalRequest{
		WithdrawalId: id, Action: action, TxHash: txHash, ReviewedBy: reviewedBy,
	})
}
