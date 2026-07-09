package azuro

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
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
		client:      &http.Client{Timeout: 2 * time.Minute},
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
	hier, err := p.FetchHierarchy(ctx, sport, params)
	if err != nil {
		return nil, err
	}

	gameIDs := make([]string, 0, len(hier.Events))
	for _, e := range hier.Events {
		gameIDs = append(gameIDs, e.ProviderID)
	}

	if len(gameIDs) > 0 {
		conds, err := p.FetchConditions(ctx, gameIDs)
		if err != nil {
			return nil, err
		}
		hier.Markets = conds.Markets
		hier.Outcomes = conds.Outcomes
	}

	return hier, nil
}

func (p *Provider) FetchHierarchy(ctx context.Context, sport string, params map[string]string) (*oddsfeed.Snapshot, error) {
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

	sportsResp, err := p.fetchSports(ctx, sportSlug, gameState)
	if err != nil {
		return nil, err
	}
	return p.buildSnapshot(&sportsResp, &conditionsResponse{}), nil
}

func (p *Provider) FetchConditions(ctx context.Context, gameIDs []string) (*oddsfeed.Snapshot, error) {
	if p.baseURL == "" {
		return nil, fmt.Errorf("azuro base URL not configured")
	}
	if len(gameIDs) == 0 {
		return &oddsfeed.Snapshot{Provider: ProviderName}, nil
	}

	condResp, err := p.fetchConditions(ctx, gameIDs)
	if err != nil {
		return nil, err
	}
	return p.buildSnapshot(&sportsResponse{}, &condResp), nil
}

func (p *Provider) SubscribeLive(ctx context.Context, sport string, updates chan<- oddsfeed.Update) error {
	if p.wsURL == "" {
		return fmt.Errorf("azuro websocket URL not configured")
	}

	condIDs, err := p.fetchLiveConditionIDs(ctx)
	if err != nil {
		return fmt.Errorf("fetch live condition ids: %w", err)
	}
	if len(condIDs) == 0 {
		<-ctx.Done()
		return ctx.Err()
	}

	dialer := websocket.Dialer{HandshakeTimeout: 10 * time.Second}
	conn, _, err := dialer.DialContext(ctx, p.wsURL, http.Header{"Accept": []string{"application/json"}})
	if err != nil {
		return fmt.Errorf("dial websocket: %w", err)
	}
	defer conn.Close()

	if err := conn.WriteJSON(map[string]interface{}{
		"action":       "subscribe",
		"conditionIds": condIDs,
	}); err != nil {
		return fmt.Errorf("subscribe: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		_, message, err := conn.ReadMessage()
		if err != nil {
			return fmt.Errorf("read message: %w", err)
		}

		var msg wsMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			continue
		}

		for _, upd := range p.updatesFromMessage(&msg) {
			select {
			case updates <- upd:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
}

// fetchSports calls Azuro's /market-manager/sports endpoint.
func (p *Provider) fetchSports(ctx context.Context, sportSlug, gameState string) (sportsResponse, error) {
	var empty sportsResponse
	reqURL := fmt.Sprintf("%s/market-manager/sports?environment=%s&gameState=%s&numberOfGames=10&orderBy=turnover&orderDirection=asc",
		p.baseURL, p.environment, gameState)
	if sportSlug != "" {
		reqURL += "&sportSlug=" + urlEncode(sportSlug)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return empty, fmt.Errorf("azuro create sports request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return empty, fmt.Errorf("azuro fetch sports: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return empty, fmt.Errorf("azuro fetch sports: status %d", resp.StatusCode)
	}

	var sportsResp sportsResponse
	if err := json.NewDecoder(resp.Body).Decode(&sportsResp); err != nil {
		return empty, fmt.Errorf("azuro decode sports: %w", err)
	}
	return sportsResp, nil
}

// fetchConditions calls Azuro's /market-manager/conditions-by-game-ids endpoint.
func (p *Provider) fetchConditions(ctx context.Context, gameIDs []string) (conditionsResponse, error) {
	var empty conditionsResponse
	body, err := json.Marshal(map[string]interface{}{
		"gameIds":     gameIDs,
		"environment": p.environment,
		"extended":    false,
	})
	if err != nil {
		return empty, fmt.Errorf("azuro marshal conditions request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/market-manager/conditions-by-game-ids", bytes.NewReader(body))
	if err != nil {
		return empty, fmt.Errorf("azuro create conditions request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return empty, fmt.Errorf("azuro fetch conditions: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return empty, fmt.Errorf("azuro fetch conditions: status %d", resp.StatusCode)
	}

	var condResp conditionsResponse
	if err := json.NewDecoder(resp.Body).Decode(&condResp); err != nil {
		return empty, fmt.Errorf("azuro decode conditions: %w", err)
	}
	return condResp, nil
}

// fetchLiveConditionIDs returns all active/paused condition IDs for currently live games.
func (p *Provider) fetchLiveConditionIDs(ctx context.Context) ([]string, error) {
	sportsResp, err := p.fetchSports(ctx, "", "Live")
	if err != nil {
		return nil, err
	}

	gameIDs := make([]string, 0, 128)
	for _, sp := range sportsResp.Sports {
		for _, c := range sp.Countries {
			for _, l := range c.Leagues {
				for _, g := range l.Games {
					gameIDs = append(gameIDs, g.GameID)
				}
			}
		}
	}

	if len(gameIDs) == 0 {
		return nil, nil
	}

	condResp, err := p.fetchConditions(ctx, gameIDs)
	if err != nil {
		return nil, err
	}

	ids := make([]string, 0, len(condResp.Conditions))
	for _, cond := range condResp.Conditions {
		id := cond.ConditionID
		if id == "" {
			id = cond.ID
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// buildSnapshot converts Azuro responses into the internal oddsfeed.Snapshot format.
func (p *Provider) buildSnapshot(sportsResp *sportsResponse, condResp *conditionsResponse) *oddsfeed.Snapshot {
	snap := &oddsfeed.Snapshot{Provider: ProviderName}

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

	// When building from conditions alone, include minimal event stubs so the
	// normalizer can resolve market/outcome parent references deterministically.
	conditionGames := make(map[string]gameRef, len(condResp.Conditions))
	for _, cond := range condResp.Conditions {
		conditionGames[cond.Game.GameID] = cond.Game
	}
	for gameID, g := range conditionGames {
		sportID := g.Sport.SportID
		if sportID == "" {
			sportID = "0"
		}
		snap.Events = append(snap.Events, oddsfeed.EventSnapshot{
			ProviderID: gameID,
			SportID:    sportID,
			Status:     "upcoming",
		})
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
			outcomeProviderID := fmt.Sprintf("%s:%s", marketID, out.OutcomeID)
			snap.Outcomes = append(snap.Outcomes, oddsfeed.OutcomeSnapshot{
				ProviderID: outcomeProviderID,
				MarketID:   marketID,
				Name:       out.Title,
				Odds:       out.Odds,
				Status:     normalizeOutcomeState(out.State),
				Metadata: map[string]string{
					"azuro_outcome_id": out.OutcomeID,
				},
			})
		}
	}

	return snap
}

// wsMessage mirrors the payload sent by Azuro's live WebSocket.
// message.data is an array of condition updates.
type wsMessage struct {
	Data []wsCondition `json:"data"`
}

type wsCondition struct {
	ID       string      `json:"id"`
	State    string      `json:"state"`
	Outcomes []wsOutcome `json:"outcomes"`
}

type wsOutcome struct {
	ID   int     `json:"id"`
	Odds float64 `json:"odds"`
}

// updatesFromMessage converts an Azuro WebSocket payload into internal Update messages.
func (p *Provider) updatesFromMessage(msg *wsMessage) []oddsfeed.Update {
	var updates []oddsfeed.Update
	for _, cond := range msg.Data {
		if cond.ID == "" {
			continue
		}

		if cond.State != "" {
			updates = append(updates, oddsfeed.Update{
				Provider: ProviderName,
				Type:     "status",
				EntityID: cond.ID,
				Payload: map[string]string{
					"status": normalizeConditionState(cond.State),
				},
			})
		}

		for _, out := range cond.Outcomes {
			outcomeID := fmt.Sprintf("%s:%d", cond.ID, out.ID)
			updates = append(updates, oddsfeed.Update{
				Provider: ProviderName,
				Type:     "odds",
				EntityID: outcomeID,
				Payload: map[string]string{
					"odds": fmt.Sprintf("%.4f", out.Odds),
				},
			})
		}
	}

	return updates
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
	case "Paused", "Stopped":
		return "suspended"
	case "Resolved":
		return "settled"
	case "Canceled", "Removed":
		return "cancelled"
	default:
		return "cancelled"
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
	case "Paused", "Stopped":
		return "suspended"
	case "Canceled", "Removed":
		return "cancelled"
	default:
		return "cancelled"
	}
}

func urlEncode(s string) string {
	return strings.ReplaceAll(s, " ", "%20")
}
