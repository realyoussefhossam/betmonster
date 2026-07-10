package gateway

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/realyoussefhossam/betmonster/internal/proto"
)

type SportsbookClient struct {
	conn pb.SportsbookServiceClient
}

func NewSportsbookClient(addr string) (*SportsbookClient, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	return &SportsbookClient{conn: pb.NewSportsbookServiceClient(conn)}, nil
}

func (c *SportsbookClient) PlaceBet(ctx context.Context, userID, eventID, marketID, outcomeID, stake, currency, referenceID string) (*pb.PlaceBetResponse, error) {
	return c.conn.PlaceBet(ctx, &pb.PlaceBetRequest{
		UserId:      userID,
		EventId:     eventID,
		MarketId:    marketID,
		OutcomeId:   outcomeID,
		Stake:       stake,
		Currency:    currency,
		ReferenceId: referenceID,
	})
}

func (c *SportsbookClient) GetBet(ctx context.Context, id string) (*pb.GetBetResponse, error) {
	return c.conn.GetBet(ctx, &pb.GetBetRequest{Id: id})
}

func (c *SportsbookClient) ListBets(ctx context.Context, userID, status string, page, pageSize int) (*pb.ListBetsResponse, error) {
	return c.conn.ListBets(ctx, &pb.ListBetsRequest{
		UserId:   userID,
		Status:   status,
		Page:     int32(page),
		PageSize: int32(pageSize),
	})
}

func (c *SportsbookClient) SettleBet(ctx context.Context, betID, outcome string) (*pb.SettleBetResponse, error) {
	return c.conn.SettleBet(ctx, &pb.SettleBetRequest{BetId: betID, Outcome: outcome})
}
