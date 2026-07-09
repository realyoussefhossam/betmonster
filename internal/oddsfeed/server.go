package oddsfeed

import (
	"context"
	"fmt"
	"time"

	pb "github.com/realyoussefhossam/betmonster/internal/proto"
)

type GRPCServer struct {
	pb.UnimplementedOddsFeedServiceServer
	service *Service
}

func NewGRPCServer(service *Service) *GRPCServer { return &GRPCServer{service: service} }

func toProtoSport(s Sport) *pb.Sport { return &pb.Sport{Id: s.ID, Name: s.Name, Slug: s.Slug} }
func toProtoLeague(l League) *pb.League {
	return &pb.League{Id: l.ID, Name: l.Name, SportId: l.SportID, Country: l.Country}
}
func toProtoEvent(e Event) *pb.Event {
	scoreUpdatedAt := ""
	if !e.ScoreUpdatedAt.IsZero() {
		scoreUpdatedAt = e.ScoreUpdatedAt.Format(time.RFC3339)
	}
	return &pb.Event{
		Id: e.ID, LeagueId: e.LeagueID, SportId: e.SportID,
		HomeParticipant: e.HomeParticipant, AwayParticipant: e.AwayParticipant,
		StartsAt: e.StartsAt.Format(time.RFC3339), Status: e.Status,
		HomeScore: e.HomeScore, AwayScore: e.AwayScore, ScoreUpdatedAt: scoreUpdatedAt,
	}
}
func toProtoMarket(m Market) *pb.Market {
	return &pb.Market{Id: m.ID, EventId: m.EventID, Type: m.Type, Name: m.Name, Line: m.Line, Status: m.Status}
}
func toProtoOutcome(o Outcome) *pb.Outcome {
	return &pb.Outcome{Id: o.ID, MarketId: o.MarketID, Name: o.Name, Odds: o.Odds, Status: o.Status}
}

func (s *GRPCServer) ListSports(ctx context.Context, req *pb.ListSportsRequest) (*pb.ListSportsResponse, error) {
	items, err := s.service.ListSports(ctx, int(req.Page), int(req.PageSize))
	if err != nil {
		return nil, fmt.Errorf("list sports: %w", err)
	}
	out := make([]*pb.Sport, len(items))
	for i, it := range items {
		out[i] = toProtoSport(it)
	}
	return &pb.ListSportsResponse{Sports: out}, nil
}

func (s *GRPCServer) ListLeagues(ctx context.Context, req *pb.ListLeaguesRequest) (*pb.ListLeaguesResponse, error) {
	items, err := s.service.ListLeagues(ctx, req.SportId, int(req.Page), int(req.PageSize))
	if err != nil {
		return nil, fmt.Errorf("list leagues: %w", err)
	}
	out := make([]*pb.League, len(items))
	for i, it := range items {
		out[i] = toProtoLeague(it)
	}
	return &pb.ListLeaguesResponse{Leagues: out}, nil
}

func (s *GRPCServer) ListEvents(ctx context.Context, req *pb.ListEventsRequest) (*pb.ListEventsResponse, error) {
	items, err := s.service.ListEvents(ctx, req.SportId, req.LeagueId, req.Status, int(req.Page), int(req.PageSize))
	if err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}
	out := make([]*pb.Event, len(items))
	for i, it := range items {
		out[i] = toProtoEvent(it)
	}
	return &pb.ListEventsResponse{Events: out}, nil
}

func (s *GRPCServer) GetEvent(ctx context.Context, req *pb.GetEventRequest) (*pb.GetEventResponse, error) {
	it, err := s.service.GetEvent(ctx, req.Id)
	if err != nil {
		return nil, fmt.Errorf("get event: %w", err)
	}
	if it == nil {
		return &pb.GetEventResponse{}, nil
	}
	return &pb.GetEventResponse{Event: toProtoEvent(*it)}, nil
}

func (s *GRPCServer) ListMarkets(ctx context.Context, req *pb.ListMarketsRequest) (*pb.ListMarketsResponse, error) {
	items, err := s.service.ListMarkets(ctx, req.EventId, req.Status, int(req.Page), int(req.PageSize))
	if err != nil {
		return nil, fmt.Errorf("list markets: %w", err)
	}
	out := make([]*pb.Market, len(items))
	for i, it := range items {
		out[i] = toProtoMarket(it)
	}
	return &pb.ListMarketsResponse{Markets: out}, nil
}

func (s *GRPCServer) ListOutcomes(ctx context.Context, req *pb.ListOutcomesRequest) (*pb.ListOutcomesResponse, error) {
	items, err := s.service.ListOutcomes(ctx, req.MarketId, req.Status, int(req.Page), int(req.PageSize))
	if err != nil {
		return nil, fmt.Errorf("list outcomes: %w", err)
	}
	out := make([]*pb.Outcome, len(items))
	for i, it := range items {
		out[i] = toProtoOutcome(it)
	}
	return &pb.ListOutcomesResponse{Outcomes: out}, nil
}

func (s *GRPCServer) ListLiveScores(ctx context.Context, req *pb.ListLiveScoresRequest) (*pb.ListLiveScoresResponse, error) {
	items, err := s.service.ListLiveScores(ctx, req.SportId, req.LeagueId, int(req.Page), int(req.PageSize))
	if err != nil {
		return nil, fmt.Errorf("list live scores: %w", err)
	}
	out := make([]*pb.Event, len(items))
	for i, it := range items {
		out[i] = toProtoEvent(it)
	}
	return &pb.ListLiveScoresResponse{Events: out}, nil
}
