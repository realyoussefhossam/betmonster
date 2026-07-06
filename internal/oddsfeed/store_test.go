package oddsfeed

import (
	"context"
	"database/sql"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func testDB(t *testing.T) *sql.DB {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		url = "postgres://wallet:wallet@localhost:5433/oddsfeed?sslmode=disable"
	}
	db, err := sql.Open("pgx", url)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.Ping(); err != nil {
		t.Fatalf("ping db: %v", err)
	}
	if err := runTestMigrations(url); err != nil {
		t.Fatalf("migrations: %v", err)
	}
	return db
}

func runTestMigrations(databaseURL string) error {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return err
	}
	defer db.Close()
	driver, err := pgx.WithInstance(db, &pgx.Config{})
	if err != nil {
		return err
	}
	_, filename, _, _ := runtime.Caller(0)
	migrationsDir := filepath.Join(filepath.Dir(filename), "migrations")
	sourceURL := url.URL{Scheme: "file", Path: migrationsDir}
	m, err := migrate.NewWithDatabaseInstance(sourceURL.String(), "pgx", driver)
	if err != nil {
		return err
	}
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return err
	}
	return nil
}

func cleanTables(t *testing.T, db *sql.DB) {
	t.Helper()
	_, err := db.Exec(`TRUNCATE odds_snapshots, outcomes, markets, events, leagues, sports RESTART IDENTITY CASCADE`)
	if err != nil {
		t.Fatalf("truncate: %v", err)
	}
}

func TestPGStoreUpsertAndList(t *testing.T) {
	db := testDB(t)
	defer db.Close()
	cleanTables(t, db)
	store := NewPGStore(db)
	ctx := context.Background()

	sportID, err := store.UpsertSport(ctx, Sport{Provider: "mock", ProviderSportID: "sp-1", Slug: "soccer", Name: "Soccer"})
	if err != nil {
		t.Fatalf("upsert sport: %v", err)
	}

	leagueID, err := store.UpsertLeague(ctx, League{Provider: "mock", ProviderLeagueID: "lg-1", SportID: sportID, Name: "League A", Country: "A"})
	if err != nil {
		t.Fatalf("upsert league: %v", err)
	}

	eventID, err := store.UpsertEvent(ctx, Event{
		Provider: "mock", ProviderEventID: "ev-1", LeagueID: leagueID, SportID: sportID,
		HomeParticipant: "A", AwayParticipant: "B", StartsAt: time.Now().Add(time.Hour), Status: "upcoming",
	})
	if err != nil {
		t.Fatalf("upsert event: %v", err)
	}

	marketID, err := store.UpsertMarket(ctx, Market{Provider: "mock", ProviderMarketID: "mk-1", EventID: eventID, Type: "1x2", Name: "Result", Status: "active"})
	if err != nil {
		t.Fatalf("upsert market: %v", err)
	}

	_, err = store.UpsertOutcome(ctx, Outcome{Provider: "mock", ProviderOutcomeID: "oc-1", MarketID: marketID, Name: "A", Odds: "2.00", Status: "active"})
	if err != nil {
		t.Fatalf("upsert outcome: %v", err)
	}

	sports, err := store.ListSports(ctx, 1, 10)
	if err != nil {
		t.Fatalf("list sports: %v", err)
	}
	if len(sports) != 1 {
		t.Fatalf("expected 1 sport, got %d", len(sports))
	}

	events, err := store.ListEvents(ctx, sportID, leagueID, "", 1, 10)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	markets, err := store.ListMarkets(ctx, eventID, "", 1, 10)
	if err != nil {
		t.Fatalf("list markets: %v", err)
	}
	if len(markets) != 1 {
		t.Fatalf("expected 1 market, got %d", len(markets))
	}
}

func TestPGStoreIdempotentUpsert(t *testing.T) {
	db := testDB(t)
	defer db.Close()
	cleanTables(t, db)
	store := NewPGStore(db)
	ctx := context.Background()

	id1, err := store.UpsertSport(ctx, Sport{Provider: "mock", ProviderSportID: "sp-1", Slug: "soccer", Name: "Soccer"})
	if err != nil {
		t.Fatalf("upsert 1: %v", err)
	}
	id2, err := store.UpsertSport(ctx, Sport{Provider: "mock", ProviderSportID: "sp-1", Slug: "soccer", Name: "Soccer"})
	if err != nil {
		t.Fatalf("upsert 2: %v", err)
	}
	if id1 != id2 {
		t.Fatalf("expected same id on upsert, got %s and %s", id1, id2)
	}
}
