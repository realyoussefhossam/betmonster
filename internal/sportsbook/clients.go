package sportsbook

import (
	"context"
	"encoding/json"
	"fmt"

	pb "github.com/realyoussefhossam/betmonster/internal/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type GRPCWalletClient struct {
	conn pb.WalletServiceClient
}

func NewGRPCWalletClient(addr string) (*GRPCWalletClient, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("dial wallet: %w", err)
	}
	return &GRPCWalletClient{conn: pb.NewWalletServiceClient(conn)}, nil
}

func (c *GRPCWalletClient) DebitWallet(ctx context.Context, userID, currency, amount, referenceID string, metadata map[string]any) error {
	metadataJSON := ""
	if metadata != nil {
		b, _ := json.Marshal(metadata)
		metadataJSON = string(b)
	}
	_, err := c.conn.DebitWallet(ctx, &pb.DebitWalletRequest{
		UserId:      userID,
		Currency:    currency,
		Amount:      amount,
		ReferenceId: referenceID,
		Metadata:    metadataJSON,
	})
	return err
}

func (c *GRPCWalletClient) CreditWallet(ctx context.Context, userID, currency, amount, referenceID string, metadata map[string]any) error {
	metadataJSON := ""
	if metadata != nil {
		b, _ := json.Marshal(metadata)
		metadataJSON = string(b)
	}
	_, err := c.conn.CreditWallet(ctx, &pb.CreditWalletRequest{
		UserId:      userID,
		Currency:    currency,
		Amount:      amount,
		ReferenceId: referenceID,
		Metadata:    metadataJSON,
	})
	return err
}

type GRPCOddsFeedClient struct {
	conn pb.OddsFeedServiceClient
}

func NewGRPCOddsFeedClient(addr string) (*GRPCOddsFeedClient, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("dial oddsfeed: %w", err)
	}
	return &GRPCOddsFeedClient{conn: pb.NewOddsFeedServiceClient(conn)}, nil
}

func (c *GRPCOddsFeedClient) GetEvent(ctx context.Context, id string) (*pb.Event, error) {
	resp, err := c.conn.GetEvent(ctx, &pb.GetEventRequest{Id: id})
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, nil
	}
	return resp.Event, nil
}

func (c *GRPCOddsFeedClient) ListMarkets(ctx context.Context, eventID, status string, page, pageSize int) ([]*pb.Market, error) {
	resp, err := c.conn.ListMarkets(ctx, &pb.ListMarketsRequest{
		EventId:  eventID,
		Status:   status,
		Page:     int32(page),
		PageSize: int32(pageSize),
	})
	if err != nil {
		return nil, err
	}
	return resp.Markets, nil
}

func (c *GRPCOddsFeedClient) ListOutcomes(ctx context.Context, marketID, status string, page, pageSize int) ([]*pb.Outcome, error) {
	resp, err := c.conn.ListOutcomes(ctx, &pb.ListOutcomesRequest{
		MarketId: marketID,
		Status:   status,
		Page:     int32(page),
		PageSize: int32(pageSize),
	})
	if err != nil {
		return nil, err
	}
	return resp.Outcomes, nil
}

func (c *GRPCOddsFeedClient) ListEvents(ctx context.Context, sportID, leagueID, status string, page, pageSize int) ([]*pb.Event, error) {
	resp, err := c.conn.ListEvents(ctx, &pb.ListEventsRequest{
		SportId:  sportID,
		LeagueId: leagueID,
		Status:   status,
		Page:     int32(page),
		PageSize: int32(pageSize),
	})
	if err != nil {
		return nil, err
	}
	return resp.Events, nil
}
