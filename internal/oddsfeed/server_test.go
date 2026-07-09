package oddsfeed_test

import (
	"context"
	"log/slog"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	pb "github.com/realyoussefhossam/betmonster/internal/proto"
	"github.com/realyoussefhossam/betmonster/internal/oddsfeed"
)

type grpcLocalMockProvider struct{}

func (p *grpcLocalMockProvider) Name() string { return "mock" }
func (p *grpcLocalMockProvider) FetchSnapshot(ctx context.Context, sport string, params map[string]string) (*oddsfeed.Snapshot, error) {
	hier, err := p.FetchHierarchy(ctx, sport, params)
	if err != nil {
		return nil, err
	}
	conds, err := p.FetchConditions(ctx, []string{"ev-1"})
	if err != nil {
		return nil, err
	}
	hier.Markets = conds.Markets
	hier.Outcomes = conds.Outcomes
	return hier, nil
}
func (p *grpcLocalMockProvider) FetchHierarchy(ctx context.Context, sport string, params map[string]string) (*oddsfeed.Snapshot, error) {
	now := time.Now()
	return &oddsfeed.Snapshot{
		Provider: "mock",
		Sports:   []oddsfeed.SportSnapshot{{ProviderID: "sp-1", Slug: "soccer", Name: "Soccer"}},
		Leagues:  []oddsfeed.LeagueSnapshot{{ProviderID: "lg-1", SportID: "sp-1", Name: "Mock League", Country: "Mockland"}},
		Events: []oddsfeed.EventSnapshot{{
			ProviderID: "ev-1", LeagueID: "lg-1", SportID: "sp-1",
			HomeParticipant: "Mock FC", AwayParticipant: "Test United",
			StartsAt: now.Add(2 * time.Hour).Format(time.RFC3339), Status: "upcoming",
		}},
	}, nil
}
func (p *grpcLocalMockProvider) FetchConditions(ctx context.Context, gameIDs []string) (*oddsfeed.Snapshot, error) {
	return &oddsfeed.Snapshot{
		Provider: "mock",
		Events: []oddsfeed.EventSnapshot{{
			ProviderID: "ev-1", SportID: "sp-1", LeagueID: "lg-1", Status: "upcoming",
		}},
		Markets: []oddsfeed.MarketSnapshot{{
			ProviderID: "mk-1", EventID: "ev-1", Type: "1x2", Name: "Match Result", Status: "active",
		}},
		Outcomes: []oddsfeed.OutcomeSnapshot{
			{ProviderID: "oc-1", MarketID: "mk-1", Name: "Home", Odds: "2.10", Status: "active"},
		},
	}, nil
}
func (p *grpcLocalMockProvider) SubscribeLive(ctx context.Context, sport string, updates chan<- oddsfeed.Update) error {
	<-ctx.Done()
	return ctx.Err()
}
func (p *grpcLocalMockProvider) ValidateConfig(cfg oddsfeed.ProviderConfig) error { return nil }

func TestGRPCServerListSports(t *testing.T) {
	ctx := context.Background()
	store := oddsfeed.NewInMemoryStore()
	svc := oddsfeed.NewService(store, []oddsfeed.FeedProvider{&grpcLocalMockProvider{}}, nil, nil, slog.Default())
	if err := svc.SyncProvider(ctx, "mock"); err != nil {
		t.Fatalf("sync: %v", err)
	}

	listener := bufconn.Listen(1024 * 1024)
	grpcServer := grpc.NewServer()
	pb.RegisterOddsFeedServiceServer(grpcServer, oddsfeed.NewGRPCServer(svc))
	go func() {
		if err := grpcServer.Serve(listener); err != nil {
			t.Errorf("grpc serve: %v", err)
		}
	}()
	defer grpcServer.Stop()

	conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	client := pb.NewOddsFeedServiceClient(conn)
	resp, err := client.ListSports(ctx, &pb.ListSportsRequest{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("list sports: %v", err)
	}
	if len(resp.Sports) != 1 {
		t.Fatalf("expected 1 sport, got %d", len(resp.Sports))
	}

	eventResp, err := client.ListEvents(ctx, &pb.ListEventsRequest{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	if len(eventResp.Events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(eventResp.Events))
	}
	if eventResp.Events[0].HomeParticipant != "Mock FC" {
		t.Fatalf("expected home participant Mock FC, got %s", eventResp.Events[0].HomeParticipant)
	}
}
