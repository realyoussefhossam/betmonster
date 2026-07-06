package oddsfeed

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJsonMapScan(t *testing.T) {
	var m jsonMap
	require.NoError(t, m.Scan([]byte(`{"key":"value"}`)))
	assert.Equal(t, "value", m["key"])

	var nilMap jsonMap
	require.NoError(t, nilMap.Scan(nil))
	assert.Nil(t, nilMap)

	var badMap jsonMap
	require.Error(t, badMap.Scan([]byte(`not-json`)))
}

func TestInMemoryStoreUpsertAndList(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	sportID, err := store.UpsertSport(ctx, Sport{Provider: "mock", ProviderSportID: "sp-1", Slug: "soccer", Name: "Soccer"})
	require.NoError(t, err)
	assert.NotEmpty(t, sportID)

	leagueID, err := store.UpsertLeague(ctx, League{Provider: "mock", ProviderLeagueID: "lg-1", SportID: sportID, Name: "League A", Country: "A"})
	require.NoError(t, err)
	assert.NotEmpty(t, leagueID)

	eventID, err := store.UpsertEvent(ctx, Event{
		Provider: "mock", ProviderEventID: "ev-1", LeagueID: leagueID, SportID: sportID,
		HomeParticipant: "A", AwayParticipant: "B", StartsAt: time.Now().Add(time.Hour), Status: "upcoming",
	})
	require.NoError(t, err)
	assert.NotEmpty(t, eventID)

	marketID, err := store.UpsertMarket(ctx, Market{Provider: "mock", ProviderMarketID: "mk-1", EventID: eventID, Type: "1x2", Name: "Result", Status: "active"})
	require.NoError(t, err)
	assert.NotEmpty(t, marketID)

	outcomeID, err := store.UpsertOutcome(ctx, Outcome{Provider: "mock", ProviderOutcomeID: "oc-1", MarketID: marketID, Name: "A", Odds: "2.00", Status: "active"})
	require.NoError(t, err)
	assert.NotEmpty(t, outcomeID)

	sports, err := store.ListSports(ctx, 1, 10)
	require.NoError(t, err)
	assert.Len(t, sports, 1)

	leagues, err := store.ListLeagues(ctx, sportID, 1, 10)
	require.NoError(t, err)
	assert.Len(t, leagues, 1)

	leaguesAll, err := store.ListLeagues(ctx, "", 1, 10)
	require.NoError(t, err)
	assert.Len(t, leaguesAll, 1)

	events, err := store.ListEvents(ctx, sportID, leagueID, "", 1, 10)
	require.NoError(t, err)
	assert.Len(t, events, 1)

	event, err := store.GetEvent(ctx, eventID)
	require.NoError(t, err)
	assert.NotNil(t, event)
	assert.Equal(t, eventID, event.ID)

	markets, err := store.ListMarkets(ctx, eventID, "", 1, 10)
	require.NoError(t, err)
	assert.Len(t, markets, 1)

	marketsActive, err := store.ListMarkets(ctx, eventID, "active", 1, 10)
	require.NoError(t, err)
	assert.Len(t, marketsActive, 1)

	outcomes, err := store.ListOutcomes(ctx, marketID, "", 1, 10)
	require.NoError(t, err)
	assert.Len(t, outcomes, 1)

	outcomesActive, err := store.ListOutcomes(ctx, marketID, "active", 1, 10)
	require.NoError(t, err)
	assert.Len(t, outcomesActive, 1)

	t.Run("filters", func(t *testing.T) {
		eventsBySport, err := store.ListEvents(ctx, sportID, "", "", 1, 10)
		require.NoError(t, err)
		assert.Len(t, eventsBySport, 1)

		eventsByStatus, err := store.ListEvents(ctx, "", "", "upcoming", 1, 10)
		require.NoError(t, err)
		assert.Len(t, eventsByStatus, 1)

		eventsMissing, err := store.ListEvents(ctx, "", "", "live", 1, 10)
		require.NoError(t, err)
		assert.Len(t, eventsMissing, 0)
	})
}

func TestInMemoryStoreIdempotentUpsert(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	id1, err := store.UpsertSport(ctx, Sport{Provider: "mock", ProviderSportID: "sp-1", Slug: "soccer", Name: "Soccer"})
	require.NoError(t, err)

	id2, err := store.UpsertSport(ctx, Sport{Provider: "mock", ProviderSportID: "sp-1", Slug: "football", Name: "Football"})
	require.NoError(t, err)
	assert.Equal(t, id1, id2)

	sports, err := store.ListSports(ctx, 1, 10)
	require.NoError(t, err)
	require.Len(t, sports, 1)
	assert.Equal(t, "Football", sports[0].Name)
	assert.Equal(t, "football", sports[0].Slug)
}

func TestInMemoryStorePagination(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		_, err := store.UpsertSport(ctx, Sport{Provider: "mock", ProviderSportID: string(rune('a' + i)), Slug: "sport", Name: string(rune('A' + i))})
		require.NoError(t, err)
	}

	page, err := store.ListSports(ctx, 1, 2)
	require.NoError(t, err)
	assert.Len(t, page, 2)

	page2, err := store.ListSports(ctx, 2, 2)
	require.NoError(t, err)
	assert.Len(t, page2, 2)

	page3, err := store.ListSports(ctx, 3, 2)
	require.NoError(t, err)
	assert.Len(t, page3, 1)

	page0, err := store.ListSports(ctx, 0, 2)
	require.NoError(t, err)
	assert.Len(t, page0, 2)
}

func TestInMemoryStoreListLiveScores(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	sportID, err := store.UpsertSport(ctx, Sport{Provider: "mock", ProviderSportID: "sp-1", Slug: "soccer", Name: "Soccer"})
	require.NoError(t, err)
	leagueID, err := store.UpsertLeague(ctx, League{Provider: "mock", ProviderLeagueID: "lg-1", SportID: sportID, Name: "League A", Country: "A"})
	require.NoError(t, err)

	_, err = store.UpsertEvent(ctx, Event{
		Provider: "mock", ProviderEventID: "ev-live", LeagueID: leagueID, SportID: sportID,
		HomeParticipant: "A", AwayParticipant: "B", StartsAt: time.Now(), Status: "live",
	})
	require.NoError(t, err)
	_, err = store.UpsertEvent(ctx, Event{
		Provider: "mock", ProviderEventID: "ev-upcoming", LeagueID: leagueID, SportID: sportID,
		HomeParticipant: "C", AwayParticipant: "D", StartsAt: time.Now().Add(time.Hour), Status: "upcoming",
	})
	require.NoError(t, err)

	live, err := store.ListLiveScores(ctx, sportID, leagueID, 1, 10)
	require.NoError(t, err)
	assert.Len(t, live, 1)
	assert.Equal(t, "live", live[0].Status)
}
