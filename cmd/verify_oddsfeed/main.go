package main

import (
	"context"
	"fmt"
	"log"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/realyoussefhossam/betmonster/internal/proto"
)

func main() {
	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "localhost:50052", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	client := pb.NewOddsFeedServiceClient(conn)

	sports, err := client.ListSports(ctx, &pb.ListSportsRequest{Page: 1, PageSize: 10})
	if err != nil {
		log.Fatalf("list sports: %v", err)
	}
	fmt.Printf("Sports (%d):\n", len(sports.Sports))
	for _, s := range sports.Sports {
		fmt.Printf("  - %s (%s, slug=%s)\n", s.Name, s.Id, s.Slug)
	}

	leagues, err := client.ListLeagues(ctx, &pb.ListLeaguesRequest{Page: 1, PageSize: 10})
	if err != nil {
		log.Fatalf("list leagues: %v", err)
	}
	fmt.Printf("\nLeagues (%d):\n", len(leagues.Leagues))
	for _, l := range leagues.Leagues {
		fmt.Printf("  - %s (%s, sport=%s, country=%s)\n", l.Name, l.Id, l.SportId, l.Country)
	}

	events, err := client.ListEvents(ctx, &pb.ListEventsRequest{Page: 1, PageSize: 10})
	if err != nil {
		log.Fatalf("list events: %v", err)
	}
	fmt.Printf("\nEvents (%d):\n", len(events.Events))
	for _, e := range events.Events {
		fmt.Printf("  - %s vs %s (%s, status=%s, starts=%s)\n", e.HomeParticipant, e.AwayParticipant, e.Id, e.Status, e.StartsAt)
	}

	markets, err := client.ListMarkets(ctx, &pb.ListMarketsRequest{Page: 1, PageSize: 10})
	if err != nil {
		log.Fatalf("list markets: %v", err)
	}
	fmt.Printf("\nMarkets (%d):\n", len(markets.Markets))
	for _, m := range markets.Markets {
		fmt.Printf("  - %s (%s, event=%s, type=%s, status=%s)\n", m.Name, m.Id, m.EventId, m.Type, m.Status)
	}

	outcomes, err := client.ListOutcomes(ctx, &pb.ListOutcomesRequest{Page: 1, PageSize: 10})
	if err != nil {
		log.Fatalf("list outcomes: %v", err)
	}
	fmt.Printf("\nOutcomes (%d):\n", len(outcomes.Outcomes))
	for _, o := range outcomes.Outcomes {
		fmt.Printf("  - %s (%s, market=%s, odds=%s, status=%s)\n", o.Name, o.Id, o.MarketId, o.Odds, o.Status)
	}
}
