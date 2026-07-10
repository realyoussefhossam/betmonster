package sportsbook

import (
	"context"
	"time"

	pb "github.com/realyoussefhossam/betmonster/internal/proto"
	"github.com/realyoussefhossam/betmonster/internal/shared/grpcmeta"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// serviceCallerID is the sentinel user id propagated by trusted internal
// services (e.g. the gateway) for privileged, non-end-user calls.
const serviceCallerID = "gateway"

// AuthInterceptor is a gRPC unary interceptor that requires every incoming
// request to include authenticated caller metadata. SettleBet is restricted to
// the trusted gateway service with admin scope; all other public methods must
// be called by the trusted gateway service.
func AuthInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing caller metadata")
	}
	userIDs := md.Get(grpcmeta.UserIDHeader)
	if len(userIDs) == 0 || userIDs[0] == "" {
		return nil, status.Error(codes.Unauthenticated, "missing caller user id")
	}
	callerID := userIDs[0]

	isAdmin := false
	if adminVals := md.Get(grpcmeta.IsAdminHeader); len(adminVals) > 0 {
		isAdmin = adminVals[0] == "true"
	}

	switch info.FullMethod {
	case pb.SportsbookService_SettleBet_FullMethodName:
		if !isAdmin || callerID != serviceCallerID {
			return nil, status.Error(codes.PermissionDenied, "admin service caller required")
		}
	default:
		// All other public methods must be called by the trusted gateway service.
		if callerID != serviceCallerID {
			return nil, status.Error(codes.PermissionDenied, "service caller required")
		}
	}

	return handler(ctx, req)
}

type GRPCServer struct {
	pb.UnimplementedSportsbookServiceServer
	svc *Service
}

func NewGRPCServer(svc *Service) *GRPCServer {
	return &GRPCServer{svc: svc}
}

func (s *GRPCServer) PlaceBet(ctx context.Context, req *pb.PlaceBetRequest) (*pb.PlaceBetResponse, error) {
	bet, err := s.svc.PlaceBet(ctx, req.UserId, req.EventId, req.MarketId, req.OutcomeId, req.Stake, req.Currency, req.ReferenceId)
	if err != nil {
		return nil, err
	}
	return &pb.PlaceBetResponse{Bet: betToProto(bet)}, nil
}

func (s *GRPCServer) GetBet(ctx context.Context, req *pb.GetBetRequest) (*pb.GetBetResponse, error) {
	bet, err := s.svc.GetBet(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	return &pb.GetBetResponse{Bet: betToProto(*bet)}, nil
}

func (s *GRPCServer) ListBets(ctx context.Context, req *pb.ListBetsRequest) (*pb.ListBetsResponse, error) {
	bets, err := s.svc.ListBets(ctx, req.UserId, req.Status, int(req.Page), int(req.PageSize))
	if err != nil {
		return nil, err
	}
	out := make([]*pb.Bet, 0, len(bets))
	for _, b := range bets {
		out = append(out, betToProto(b))
	}
	return &pb.ListBetsResponse{Bets: out}, nil
}

func (s *GRPCServer) SettleBet(ctx context.Context, req *pb.SettleBetRequest) (*pb.SettleBetResponse, error) {
	bet, err := s.svc.SettleBet(ctx, req.BetId, req.Outcome)
	if err != nil {
		return nil, err
	}
	return &pb.SettleBetResponse{Bet: betToProto(bet)}, nil
}

func betToProto(b Bet) *pb.Bet {
	settledAt := ""
	if b.SettledAt != nil {
		settledAt = b.SettledAt.Format(time.RFC3339)
	}
	return &pb.Bet{
		Id:              b.ID,
		UserId:          b.UserID,
		EventId:         b.EventID,
		MarketId:        b.MarketID,
		OutcomeId:       b.OutcomeID,
		Odds:            b.Odds,
		Stake:           b.Stake,
		PotentialPayout: b.PotentialPayout,
		Currency:        b.Currency,
		Status:          b.Status,
		ReferenceId:     b.ReferenceID,
		CreatedAt:       b.CreatedAt.Format(time.RFC3339),
		SettledAt:       settledAt,
	}
}
