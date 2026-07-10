package sportsbook

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	pb "github.com/realyoussefhossam/betmonster/internal/proto"
	"github.com/realyoussefhossam/betmonster/internal/shared/grpcmeta"
)

func serviceCtx() context.Context {
	return metadata.AppendToOutgoingContext(context.Background(),
		grpcmeta.UserIDHeader, "gateway",
		grpcmeta.IsAdminHeader, "true",
	)
}

func endUserCtx(userID string) context.Context {
	return metadata.AppendToOutgoingContext(context.Background(),
		grpcmeta.UserIDHeader, userID,
	)
}

func newSportsbookServer(t *testing.T) (pb.SportsbookServiceClient, *mockWalletClient, *mockOddsFeedClient) {
	t.Helper()
	store := NewInMemoryStore()
	wallet := &mockWalletClient{}
	oddsfeed := newMockOddsFeedClient()
	svc := NewService(store, wallet, oddsfeed)
	grpcServer := grpc.NewServer(grpc.UnaryInterceptor(AuthInterceptor))
	pb.RegisterSportsbookServiceServer(grpcServer, NewGRPCServer(svc))

	listener := bufconn.Listen(1024 * 1024)
	go func() {
		if err := grpcServer.Serve(listener); err != nil {
			t.Logf("grpc server error: %v", err)
		}
	}()
	t.Cleanup(grpcServer.Stop)

	conn, err := grpc.Dial("bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
		return listener.Dial()
	}), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })

	return pb.NewSportsbookServiceClient(conn), wallet, oddsfeed
}

func TestGRPCPlaceBet(t *testing.T) {
	client, wallet, oddsfeed := newSportsbookServer(t)
	oddsfeed.seedEvent("event-1", "market-1", "outcome-1", "2.20")

	ctx, cancel := context.WithTimeout(serviceCtx(), 5*time.Second)
	defer cancel()
	resp, err := client.PlaceBet(ctx, &pb.PlaceBetRequest{
		UserId:      "user-1",
		EventId:     "event-1",
		MarketId:    "market-1",
		OutcomeId:   "outcome-1",
		Stake:       "5.00",
		Currency:    "USDT",
		ReferenceId: "ref-g-1",
	})
	require.NoError(t, err)
	require.NotNil(t, resp.Bet)
	assert.Equal(t, "user-1", resp.Bet.UserId)
	assert.Equal(t, "11", resp.Bet.PotentialPayout)
	assert.Equal(t, StatusPending, resp.Bet.Status)
	assert.Len(t, wallet.debits, 1)
}

func TestGRPCPlaceBetRejectsEndUserCaller(t *testing.T) {
	client, _, oddsfeed := newSportsbookServer(t)
	oddsfeed.seedEvent("event-1", "market-1", "outcome-1", "2.00")

	ctx, cancel := context.WithTimeout(endUserCtx("user-1"), 5*time.Second)
	defer cancel()
	_, err := client.PlaceBet(ctx, &pb.PlaceBetRequest{
		UserId:      "user-1",
		EventId:     "event-1",
		MarketId:    "market-1",
		OutcomeId:   "outcome-1",
		Stake:       "5.00",
		Currency:    "USDT",
		ReferenceId: "ref-g-reject",
	})
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.PermissionDenied, st.Code())
}

func TestGRPCGetBet(t *testing.T) {
	client, _, oddsfeed := newSportsbookServer(t)
	oddsfeed.seedEvent("event-1", "market-1", "outcome-1", "2.00")

	ctx, cancel := context.WithTimeout(serviceCtx(), 5*time.Second)
	defer cancel()
	placed, err := client.PlaceBet(ctx, &pb.PlaceBetRequest{
		UserId:      "user-1",
		EventId:     "event-1",
		MarketId:    "market-1",
		OutcomeId:   "outcome-1",
		Stake:       "5.00",
		Currency:    "USDT",
		ReferenceId: "ref-g-2",
	})
	require.NoError(t, err)

	got, err := client.GetBet(ctx, &pb.GetBetRequest{Id: placed.Bet.Id})
	require.NoError(t, err)
	assert.Equal(t, placed.Bet.Id, got.Bet.Id)
}

func TestGRPCListBets(t *testing.T) {
	client, _, oddsfeed := newSportsbookServer(t)
	oddsfeed.seedEvent("event-1", "market-1", "outcome-1", "2.00")

	ctx, cancel := context.WithTimeout(serviceCtx(), 5*time.Second)
	defer cancel()
	_, err := client.PlaceBet(ctx, &pb.PlaceBetRequest{
		UserId:      "user-1",
		EventId:     "event-1",
		MarketId:    "market-1",
		OutcomeId:   "outcome-1",
		Stake:       "5.00",
		Currency:    "USDT",
		ReferenceId: "ref-g-3",
	})
	require.NoError(t, err)

	list, err := client.ListBets(ctx, &pb.ListBetsRequest{UserId: "user-1"})
	require.NoError(t, err)
	require.Len(t, list.Bets, 1)
	assert.Equal(t, "user-1", list.Bets[0].UserId)
}

func TestGRPCSettleBet(t *testing.T) {
	client, wallet, oddsfeed := newSportsbookServer(t)
	oddsfeed.seedEvent("event-1", "market-1", "outcome-1", "2.00")

	ctx, cancel := context.WithTimeout(serviceCtx(), 5*time.Second)
	defer cancel()
	placed, err := client.PlaceBet(ctx, &pb.PlaceBetRequest{
		UserId:      "user-1",
		EventId:     "event-1",
		MarketId:    "market-1",
		OutcomeId:   "outcome-1",
		Stake:       "5.00",
		Currency:    "USDT",
		ReferenceId: "ref-g-4",
	})
	require.NoError(t, err)

	settled, err := client.SettleBet(ctx, &pb.SettleBetRequest{BetId: placed.Bet.Id, Outcome: StatusWon})
	require.NoError(t, err)
	assert.Equal(t, StatusWon, settled.Bet.Status)
	assert.Len(t, wallet.credits, 1)
	assert.Equal(t, "10", wallet.credits[0].amount)
}

func TestGRPCSettleBetRejectsNonAdminCaller(t *testing.T) {
	client, _, oddsfeed := newSportsbookServer(t)
	oddsfeed.seedEvent("event-1", "market-1", "outcome-1", "2.00")

	placeCtx, cancel := context.WithTimeout(serviceCtx(), 5*time.Second)
	defer cancel()
	placed, err := client.PlaceBet(placeCtx, &pb.PlaceBetRequest{
		UserId:      "user-1",
		EventId:     "event-1",
		MarketId:    "market-1",
		OutcomeId:   "outcome-1",
		Stake:       "5.00",
		Currency:    "USDT",
		ReferenceId: "ref-g-settle-reject",
	})
	require.NoError(t, err)

	// Gateway caller without admin flag should not be allowed to settle.
	settleCtx, settleCancel := context.WithTimeout(
		metadata.AppendToOutgoingContext(context.Background(), grpcmeta.UserIDHeader, "gateway"),
		5*time.Second,
	)
	defer settleCancel()
	_, err = client.SettleBet(settleCtx, &pb.SettleBetRequest{BetId: placed.Bet.Id, Outcome: StatusWon})
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.PermissionDenied, st.Code())
}
