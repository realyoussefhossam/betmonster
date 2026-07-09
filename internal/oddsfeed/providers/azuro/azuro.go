package azuro

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/realyoussefhossam/betmonster/internal/oddsfeed"
)

const ProviderName = "azuro"

// Provider implements the oddsfeed.FeedProvider interface for the Azuro Protocol.
// It consumes Azuro's public REST API (market-manager endpoints) for snapshots
// and the public WebSocket stream for live odds updates.
type Provider struct {
	baseURL     string
	wsURL       string
	environment string
	client      *http.Client
}

func New(baseURL, wsURL, environment string) *Provider {
	if environment == "" {
		environment = "PolygonUSDT"
	}
	if baseURL != "" {
		baseURL = strings.TrimSuffix(baseURL, "/")
	}
	return &Provider{
		baseURL:     baseURL,
		wsURL:       wsURL,
		environment: environment,
		client:      &http.Client{Timeout: 30 * time.Second},
	}
}

func (p *Provider) Name() string { return ProviderName }

func (p *Provider) ValidateConfig(cfg oddsfeed.ProviderConfig) error {
	if cfg.GraphURL == "" {
		return fmt.Errorf("graph URL required")
	}
	return nil
}

// sportsResponse mirrors Azuro's /market-manager/sports payload.
type sportsResponse struct {
	Sports []sport `json:"sports"`
}

type sport struct {
	ID      int        `json:"id"`
	Slug    string     `json:"slug"`
	Name    string     `json:"name"`
	SportID string     `json:"sportId"`
	Countries []country `json:"countries"`
}

type country struct {
	Slug    string   `json:"slug"`
	Name    string   `json:"name"`
	Leagues []league `json:"leagues"`
}

type league struct {
	Slug   string `json:"slug"`
	Name   string `json:"name"`
	Games  []game `json:"games"`
}

type game struct {
	GameID    string        `json:"gameId"`
	Slug      string        `json:"slug"`
	Title     string        `json:"title"`
	StartsAt  string        `json:"startsAt"`
	State     string        `json:"state"`
	Sport     sportRef      `json:"sport"`
	League    leagueRef     `json:"league"`
	Country   countryRef    `json:"country"`
	Participants []participant `json:"participants"`
}

type sportRef struct {
	SportID string `json:"sportId"`
	Slug    string `json:"slug"`
	Name    string `json:"name"`
}

type leagueRef struct {
	ID   string `json:"id"`
	Slug string `json:"slug"`
	Name string `json:"name"`
}

type countryRef struct {
	ID   string `json:"id"`
	Slug string `json:"slug"`
	Name string `json:"name"`
}

type participant struct {
	Name string `json:"name"`
}

// conditionsResponse mirrors Azuro's /market-manager/conditions-by-game-ids payload.
type conditionsResponse struct {
	Conditions []condition `json:"conditions"`
}

type condition struct {
	ID          string    `json:"id"`
	ConditionID string    `json:"conditionId"`
	State       string    `json:"state"`
	Title       string    `json:"title"`
	Game        gameRef   `json:"game"`
	Outcomes    []outcome `json:"outcomes"`
}

type gameRef struct {
	GameID string  `json:"gameId"`
	Sport  sportRef `json:"sport"`
}

type outcome struct {
	Title    string `json:"title"`
	OutcomeID string `json:"outcomeId"`
	Odds     string `json:"odds"`
	State    string `json:"state"`
}

func (p *Provider) FetchSnapshot(ctx context.Context, sport string, params map[string]string) (*oddsfeed.Snapshot, error) {
	if p.baseURL == "" {
		return nil, fmt.Errorf("azuro base URL not configured")
	}

	sportSlug := sport
	if sportSlug == "" {
		sportSlug = params["sport_slug"]
	}

	gameState := "Prematch"
	if params["game_state"] != "" {
		gameState = params["game_state"]
	}

	reqURL := fmt.Sprintf("%s/market-manager/sports?environment=%s&gameState=%s&numberOfGames=10&orderBy=turnover&orderDirection=asc",
		p.baseURL, p.environment, gameState)
	if sportSlug != "" {
		reqURL += "&sportSlug=" + urlEncode(sportSlug)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("azuro create sports request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("azuro fetch sports: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("azuro fetch sports: status %d", resp.StatusCode)
	}

	var sportsResp sportsResponse
	if err := json.NewDecoder(resp.Body).Decode(&sportsResp); err != nil {
		return nil, fmt.Errorf("azuro decode sports: %w", err)
	}

	snap := &oddsfeed.Snapshot{Provider: ProviderName}
	gameIDs := make([]string, 0, 128)

	for _, sp := range sportsResp.Sports {
		sportID := providerSportID(sp)
		snap.Sports = append(snap.Sports, oddsfeed.SportSnapshot{
			ProviderID: sportID,
			Slug:       sp.Slug,
			Name:       sp.Name,
		})

		for _, c := range sp.Countries {
			for _, l := range c.Leagues {
				leagueID := providerLeagueID(sp, c, l)
				snap.Leagues = append(snap.Leagues, oddsfeed.LeagueSnapshot{
					ProviderID: leagueID,
					SportID:    sportID,
					Name:       l.Name,
					Country:    c.Name,
				})

				for _, g := range l.Games {
					eventID := g.GameID
					gameIDs = append(gameIDs, eventID)
					snap.Events = append(snap.Events, oddsfeed.EventSnapshot{
						ProviderID:      eventID,
						LeagueID:        leagueID,
						SportID:         sportID,
						HomeParticipant: homeParticipant(g.Participants),
						AwayParticipant: awayParticipant(g.Participants),
						StartsAt:        azuroTime(g.StartsAt),
						Status:          normalizeGameState(g.State),
						Metadata: map[string]string{
							"title": g.Title,
							"slug":  g.Slug,
						},
					})
				}
			}
		}
	}

	if len(gameIDs) == 0 {
		return snap, nil
	}

	conditionsReqBody, err := json.Marshal(map[string]interface{}{
		"gameIds":     gameIDs,
		"environment": p.environment,
		"extended":    false,
	})
	if err != nil {
		return nil, fmt.Errorf("azuro marshal conditions request: %w", err)
	}

	conditionsURL := p.baseURL + "/market-manager/conditions-by-game-ids"
	postReq, err := http.NewRequestWithContext(ctx, http.MethodPost, conditionsURL, bytes.NewReader(conditionsReqBody))
	if err != nil {
		return nil, fmt.Errorf("azuro create conditions request: %w", err)
	}
	postReq.Header.Set("Accept", "application/json")
	postReq.Header.Set("Content-Type", "application/json")

	postResp, err := p.client.Do(postReq)
	if err != nil {
		return nil, fmt.Errorf("azuro fetch conditions: %w", err)
	}
	defer postResp.Body.Close()
	if postResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("azuro fetch conditions: status %d", postResp.StatusCode)
	}

	var condResp conditionsResponse
	if err := json.NewDecoder(postResp.Body).Decode(&condResp); err != nil {
		return nil, fmt.Errorf("azuro decode conditions: %w", err)
	}

	for _, cond := range condResp.Conditions {
		marketID := cond.ConditionID
		if marketID == "" {
			marketID = cond.ID
		}
		snap.Markets = append(snap.Markets, oddsfeed.MarketSnapshot{
			ProviderID: marketID,
			EventID:    cond.Game.GameID,
			Type:       cond.Title,
			Name:       cond.Title,
			Status:     normalizeConditionState(cond.State),
			Metadata: map[string]string{
				"condition_id": cond.ID,
			},
		})

		for _, out := range cond.Outcomes {
			snap.Outcomes = append(snap.Outcomes, oddsfeed.OutcomeSnapshot{
				ProviderID: out.OutcomeID,
				MarketID:   marketID,
				Name:       out.Title,
				Odds:       out.Odds,
				Status:     normalizeOutcomeState(out.State),
				Metadata:   map[string]string{},
			})
		}
	}

	return snap, nil
}

func (p *Provider) SubscribeLive(ctx context.Context, sport string, updates chan<- oddsfeed.Update) error {
	if p.wsURL == "" {
		return fmt.Errorf("azuro websocket URL not configured")
	}
	// TODO: connect to Azuro WebSocket and push oddsfeed.Update messages.
	<-ctx.Done()
	return ctx.Err()
}

func providerSportID(sp sport) string {
	if sp.SportID != "" {
		return sp.SportID
	}
	return fmt.Sprintf("%d", sp.ID)
}

func providerLeagueID(sp sport, c country, l league) string {
	return fmt.Sprintf("%s:%s:%s", providerSportID(sp), c.Slug, l.Slug)
}

func homeParticipant(parts []participant) string {
	if len(parts) > 0 {
		return parts[0].Name
	}
	return ""
}

func awayParticipant(parts []participant) string {
	if len(parts) > 1 {
		return parts[1].Name
	}
	return ""
}

func azuroTime(ts string) string {
	// Azuro returns Unix timestamps as strings; convert to RFC3339.
	if ts == "" {
		return ""
	}
	sec, err := parseInt64(ts)
	if err != nil {
		return ts
	}
	return time.Unix(sec, 0).UTC().Format(time.RFC3339)
}

func parseInt64(s string) (int64, error) {
	var n int64
	_, err := fmt.Sscanf(s, "%d", &n)
	return n, err
}

func normalizeGameState(state string) string {
	switch state {
	case "Prematch":
		return "upcoming"
	case "Live":
		return "live"
	case "Finished":
		return "finished"
	case "Canceled":
		return "canceled"
	default:
		return strings.ToLower(state)
	}
}

func normalizeConditionState(state string) string {
	switch state {
	case "Active":
		return "active"
	case "Paused":
		return "paused"
	case "Resolved":
		return "resolved"
	case "Canceled":
		return "canceled"
	default:
		return strings.ToLower(state)
	}
}

func normalizeOutcomeState(state string) string {
	switch state {
	case "Active":
		return "active"
	case "Won":
		return "won"
	case "Lost":
		return "lost"
	default:
		return strings.ToLower(state)
	}
}

func urlEncode(s string) string {
	return strings.ReplaceAll(s, " ", "%20")
}
