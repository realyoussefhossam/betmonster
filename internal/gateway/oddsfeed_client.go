package gateway

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/realyoussefhossam/betmonster/internal/proto"
)

type OddsFeedClient struct {
	conn pb.OddsFeedServiceClient
}

func NewOddsFeedClient(addr string) (*OddsFeedClient, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	return &OddsFeedClient{conn: pb.NewOddsFeedServiceClient(conn)}, nil
}

func (c *OddsFeedClient) ListSports(ctx context.Context, page, pageSize int) (*pb.ListSportsResponse, error) {
	return c.conn.ListSports(ctx, &pb.ListSportsRequest{Page: int32(page), PageSize: int32(pageSize)})
}

func (c *OddsFeedClient) ListLeagues(ctx context.Context, sportID string, page, pageSize int) (*pb.ListLeaguesResponse, error) {
	return c.conn.ListLeagues(ctx, &pb.ListLeaguesRequest{SportId: sportID, Page: int32(page), PageSize: int32(pageSize)})
}

func (c *OddsFeedClient) ListEvents(ctx context.Context, sportID, leagueID, status string, page, pageSize int) (*pb.ListEventsResponse, error) {
	return c.conn.ListEvents(ctx, &pb.ListEventsRequest{SportId: sportID, LeagueId: leagueID, Status: status, Page: int32(page), PageSize: int32(pageSize)})
}

func (c *OddsFeedClient) GetEvent(ctx context.Context, id string) (*pb.GetEventResponse, error) {
	return c.conn.GetEvent(ctx, &pb.GetEventRequest{Id: id})
}

func (c *OddsFeedClient) ListMarkets(ctx context.Context, eventID, status string, page, pageSize int) (*pb.ListMarketsResponse, error) {
	return c.conn.ListMarkets(ctx, &pb.ListMarketsRequest{EventId: eventID, Status: status, Page: int32(page), PageSize: int32(pageSize)})
}

func (c *OddsFeedClient) ListOutcomes(ctx context.Context, marketID, status string, page, pageSize int) (*pb.ListOutcomesResponse, error) {
	return c.conn.ListOutcomes(ctx, &pb.ListOutcomesRequest{MarketId: marketID, Status: status, Page: int32(page), PageSize: int32(pageSize)})
}

func (c *OddsFeedClient) ListLiveScores(ctx context.Context, sportID, leagueID string, page, pageSize int) (*pb.ListLiveScoresResponse, error) {
	return c.conn.ListLiveScores(ctx, &pb.ListLiveScoresRequest{SportId: sportID, LeagueId: leagueID, Page: int32(page), PageSize: int32(pageSize)})
}
