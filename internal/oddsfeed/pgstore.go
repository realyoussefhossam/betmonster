package oddsfeed

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
)

// jsonMap is a sql.Scanner adapter for nullable JSONB metadata columns.
type jsonMap map[string]string

func (m *jsonMap) Scan(value interface{}) error {
	if value == nil {
		*m = nil
		return nil
	}
	var b []byte
	switch v := value.(type) {
	case []byte:
		b = v
	case string:
		b = []byte(v)
	default:
		return fmt.Errorf("cannot scan %T into jsonMap", value)
	}
	if err := json.Unmarshal(b, (*map[string]string)(m)); err != nil {
		return fmt.Errorf("unmarshal jsonMap: %w", err)
	}
	return nil
}

type PGStore struct {
	db *sql.DB
}

func NewPGStore(db *sql.DB) *PGStore { return &PGStore{db: db} }

func parseUUID(s string) (uuid.UUID, error) {
	if s == "" {
		return uuid.Nil, nil
	}
	return uuid.Parse(s)
}

func (s *PGStore) UpsertSport(ctx context.Context, sp Sport) (string, error) {
	sportUUID, err := uuid.Parse(sp.ID)
	if err != nil {
		return "", fmt.Errorf("upsert sport: invalid id uuid: %w", err)
	}
	var id string
	err = s.db.QueryRowContext(ctx, `
		INSERT INTO sports (id, provider, provider_sport_id, slug, name)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (provider, provider_sport_id) DO UPDATE SET
			slug = EXCLUDED.slug,
			name = EXCLUDED.name,
			updated_at = now()
		RETURNING id
	`, sportUUID, sp.Provider, sp.ProviderSportID, sp.Slug, sp.Name).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("upsert sport: %w", err)
	}
	return id, nil
}

func (s *PGStore) UpsertLeague(ctx context.Context, l League) (string, error) {
	leagueUUID, err := uuid.Parse(l.ID)
	if err != nil {
		return "", fmt.Errorf("upsert league: invalid id uuid: %w", err)
	}
	sportUUID, err := parseUUID(l.SportID)
	if err != nil {
		return "", fmt.Errorf("upsert league: invalid sport uuid: %w", err)
	}
	var id string
	err = s.db.QueryRowContext(ctx, `
		INSERT INTO leagues (id, provider, provider_league_id, sport_id, name, country)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (provider, provider_league_id) DO UPDATE SET
			sport_id = EXCLUDED.sport_id,
			name = EXCLUDED.name,
			country = EXCLUDED.country,
			updated_at = now()
		RETURNING id
	`, leagueUUID, l.Provider, l.ProviderLeagueID, sportUUID, l.Name, l.Country).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("upsert league: %w", err)
	}
	return id, nil
}

func (s *PGStore) UpsertEvent(ctx context.Context, e Event) (string, error) {
	eventUUID, err := uuid.Parse(e.ID)
	if err != nil {
		return "", fmt.Errorf("upsert event: invalid id uuid: %w", err)
	}
	leagueUUID, err := parseUUID(e.LeagueID)
	if err != nil {
		return "", fmt.Errorf("upsert event: invalid league uuid: %w", err)
	}
	sportUUID, err := parseUUID(e.SportID)
	if err != nil {
		return "", fmt.Errorf("upsert event: invalid sport uuid: %w", err)
	}
	var id string
	scoreUpdated := sql.NullTime{Time: e.ScoreUpdatedAt, Valid: !e.ScoreUpdatedAt.IsZero()}
	err = s.db.QueryRowContext(ctx, `
		INSERT INTO events (id, provider, provider_event_id, league_id, sport_id, home_participant, away_participant, starts_at, status, home_score, away_score, score_updated_at, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		ON CONFLICT (provider, provider_event_id) DO UPDATE SET
			league_id = EXCLUDED.league_id,
			sport_id = EXCLUDED.sport_id,
			home_participant = EXCLUDED.home_participant,
			away_participant = EXCLUDED.away_participant,
			starts_at = EXCLUDED.starts_at,
			status = EXCLUDED.status,
			home_score = EXCLUDED.home_score,
			away_score = EXCLUDED.away_score,
			score_updated_at = EXCLUDED.score_updated_at,
			metadata = EXCLUDED.metadata,
			updated_at = now()
		RETURNING id
	`, eventUUID, e.Provider, e.ProviderEventID, leagueUUID, sportUUID, e.HomeParticipant, e.AwayParticipant, e.StartsAt, e.Status, e.HomeScore, e.AwayScore, scoreUpdated, e.Metadata).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("upsert event: %w", err)
	}
	return id, nil
}

func (s *PGStore) UpsertMarket(ctx context.Context, m Market) (string, error) {
	marketUUID, err := uuid.Parse(m.ID)
	if err != nil {
		return "", fmt.Errorf("upsert market: invalid id uuid: %w", err)
	}
	eventUUID, err := parseUUID(m.EventID)
	if err != nil {
		return "", fmt.Errorf("upsert market: invalid event uuid: %w", err)
	}
	var id string
	err = s.db.QueryRowContext(ctx, `
		INSERT INTO markets (id, provider, provider_market_id, event_id, type, name, line, status, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (provider, provider_market_id) DO UPDATE SET
			event_id = EXCLUDED.event_id,
			type = EXCLUDED.type,
			name = EXCLUDED.name,
			line = EXCLUDED.line,
			status = EXCLUDED.status,
			metadata = EXCLUDED.metadata,
			updated_at = now()
		RETURNING id
	`, marketUUID, m.Provider, m.ProviderMarketID, eventUUID, m.Type, m.Name, m.Line, m.Status, m.Metadata).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("upsert market: %w", err)
	}
	return id, nil
}

func (s *PGStore) UpsertOutcome(ctx context.Context, o Outcome) (string, error) {
	outcomeUUID, err := uuid.Parse(o.ID)
	if err != nil {
		return "", fmt.Errorf("upsert outcome: invalid id uuid: %w", err)
	}
	marketUUID, err := parseUUID(o.MarketID)
	if err != nil {
		return "", fmt.Errorf("upsert outcome: invalid market uuid: %w", err)
	}
	var id string
	err = s.db.QueryRowContext(ctx, `
		INSERT INTO outcomes (id, provider, provider_outcome_id, market_id, name, odds, status, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (provider, provider_outcome_id) DO UPDATE SET
			market_id = EXCLUDED.market_id,
			name = EXCLUDED.name,
			odds = EXCLUDED.odds,
			status = EXCLUDED.status,
			metadata = EXCLUDED.metadata,
			updated_at = now()
		RETURNING id
	`, outcomeUUID, o.Provider, o.ProviderOutcomeID, marketUUID, o.Name, o.Odds, o.Status, o.Metadata).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("upsert outcome: %w", err)
	}
	return id, nil
}

func (s *PGStore) UpdateOutcomeOdds(ctx context.Context, provider, providerOutcomeID, odds string) (string, string, error) {
	var marketID, outcomeID string
	err := s.db.QueryRowContext(ctx, `
		UPDATE outcomes
		SET odds = $3, updated_at = now()
		WHERE provider = $1 AND provider_outcome_id = $2
		RETURNING id, market_id
	`, provider, providerOutcomeID, odds).Scan(&outcomeID, &marketID)
	if err == sql.ErrNoRows {
		return "", "", fmt.Errorf("update outcome odds: not found")
	}
	if err != nil {
		return "", "", fmt.Errorf("update outcome odds: %w", err)
	}
	return marketID, outcomeID, nil
}

func (s *PGStore) UpdateMarketStatus(ctx context.Context, provider, providerMarketID, status string) (string, error) {
	var marketID string
	err := s.db.QueryRowContext(ctx, `
		UPDATE markets
		SET status = $3, updated_at = now()
		WHERE provider = $1 AND provider_market_id = $2
		RETURNING id
	`, provider, providerMarketID, status).Scan(&marketID)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("update market status: not found")
	}
	if err != nil {
		return "", fmt.Errorf("update market status: %w", err)
	}
	return marketID, nil
}

func (s *PGStore) UpdateOutcomeStatus(ctx context.Context, provider, providerOutcomeID, status string) (string, string, error) {
	var marketID, outcomeID string
	err := s.db.QueryRowContext(ctx, `
		UPDATE outcomes
		SET status = $3, updated_at = now()
		WHERE provider = $1 AND provider_outcome_id = $2
		RETURNING id, market_id
	`, provider, providerOutcomeID, status).Scan(&outcomeID, &marketID)
	if err == sql.ErrNoRows {
		return "", "", fmt.Errorf("update outcome status: not found")
	}
	if err != nil {
		return "", "", fmt.Errorf("update outcome status: %w", err)
	}
	return marketID, outcomeID, nil
}

func (s *PGStore) UpdateEventStatus(ctx context.Context, provider, providerEventID, status string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE events
		SET status = $1, updated_at = now()
		WHERE provider = $2 AND provider_event_id = $3
	`, status, provider, providerEventID)
	return err
}

func (s *PGStore) GetEventStatusesByProvider(ctx context.Context, provider string) (map[string]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT provider_event_id, status FROM events WHERE provider = $1`, provider)
	if err != nil {
		return nil, fmt.Errorf("get event statuses: %w", err)
	}
	defer rows.Close()

	statuses := make(map[string]string)
	for rows.Next() {
		var providerEventID, status string
		if err := rows.Scan(&providerEventID, &status); err != nil {
			return nil, fmt.Errorf("get event statuses: scan: %w", err)
		}
		statuses[providerEventID] = status
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("get event statuses: rows: %w", err)
	}
	return statuses, nil
}

func (s *PGStore) ListSports(ctx context.Context, page, pageSize int) ([]Sport, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	rows, err := s.db.QueryContext(ctx, `SELECT id, provider, provider_sport_id, slug, name, created_at, updated_at FROM sports ORDER BY name LIMIT $1 OFFSET $2`, pageSize, offset)
	if err != nil {
		return nil, fmt.Errorf("list sports: %w", err)
	}
	defer rows.Close()
	var out []Sport
	for rows.Next() {
		var sp Sport
		if err := rows.Scan(&sp.ID, &sp.Provider, &sp.ProviderSportID, &sp.Slug, &sp.Name, &sp.CreatedAt, &sp.UpdatedAt); err != nil {
			return nil, fmt.Errorf("list sports: scan: %w", err)
		}
		out = append(out, sp)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list sports: rows: %w", err)
	}
	return out, nil
}

func (s *PGStore) ListLeagues(ctx context.Context, sportID string, page, pageSize int) ([]League, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	var sportUUID interface{} = nil
	if sportID != "" {
		parsed, err := uuid.Parse(sportID)
		if err != nil {
			return nil, fmt.Errorf("list leagues: invalid sport uuid: %w", err)
		}
		sportUUID = parsed
	}

	rows, err := s.db.QueryContext(ctx, `SELECT id, provider, provider_league_id, sport_id, name, country, created_at, updated_at FROM leagues WHERE ($1::uuid IS NULL OR sport_id = $1::uuid) ORDER BY name LIMIT $2 OFFSET $3`, sportUUID, pageSize, offset)
	if err != nil {
		return nil, fmt.Errorf("list leagues: %w", err)
	}
	defer rows.Close()
	var out []League
	for rows.Next() {
		var l League
		if err := rows.Scan(&l.ID, &l.Provider, &l.ProviderLeagueID, &l.SportID, &l.Name, &l.Country, &l.CreatedAt, &l.UpdatedAt); err != nil {
			return nil, fmt.Errorf("list leagues: scan: %w", err)
		}
		out = append(out, l)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list leagues: rows: %w", err)
	}
	return out, nil
}

func (s *PGStore) ListEvents(ctx context.Context, sportID, leagueID, status string, page, pageSize int) ([]Event, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	var sportUUID interface{} = nil
	if sportID != "" {
		parsed, err := uuid.Parse(sportID)
		if err != nil {
			return nil, fmt.Errorf("list events: invalid sport uuid: %w", err)
		}
		sportUUID = parsed
	}
	var leagueUUID interface{} = nil
	if leagueID != "" {
		parsed, err := uuid.Parse(leagueID)
		if err != nil {
			return nil, fmt.Errorf("list events: invalid league uuid: %w", err)
		}
		leagueUUID = parsed
	}

	query := `SELECT id, provider, provider_event_id, league_id, sport_id, home_participant, away_participant, starts_at, status, home_score, away_score, score_updated_at, metadata, created_at, updated_at FROM events
		WHERE ($1::uuid IS NULL OR sport_id = $1::uuid)
		  AND ($2::uuid IS NULL OR league_id = $2::uuid)
		  AND ($3 = '' OR status = $3)
		ORDER BY starts_at DESC LIMIT $4 OFFSET $5`
	rows, err := s.db.QueryContext(ctx, query, sportUUID, leagueUUID, status, pageSize, offset)
	if err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}
	defer rows.Close()
	var out []Event
	for rows.Next() {
		var e Event
		var scoreUpdated sql.NullTime
		if err := rows.Scan(&e.ID, &e.Provider, &e.ProviderEventID, &e.LeagueID, &e.SportID, &e.HomeParticipant, &e.AwayParticipant, &e.StartsAt, &e.Status, &e.HomeScore, &e.AwayScore, &scoreUpdated, (*jsonMap)(&e.Metadata), &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, fmt.Errorf("list events: scan: %w", err)
		}
		if scoreUpdated.Valid {
			e.ScoreUpdatedAt = scoreUpdated.Time
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list events: rows: %w", err)
	}
	return out, nil
}

func (s *PGStore) GetEvent(ctx context.Context, id string) (*Event, error) {
	eventUUID, err := parseUUID(id)
	if err != nil {
		return nil, fmt.Errorf("get event: invalid uuid: %w", err)
	}
	var e Event
	var scoreUpdated sql.NullTime
	err = s.db.QueryRowContext(ctx, `SELECT id, provider, provider_event_id, league_id, sport_id, home_participant, away_participant, starts_at, status, home_score, away_score, score_updated_at, metadata, created_at, updated_at FROM events WHERE id = $1`, eventUUID).Scan(
		&e.ID, &e.Provider, &e.ProviderEventID, &e.LeagueID, &e.SportID, &e.HomeParticipant, &e.AwayParticipant, &e.StartsAt, &e.Status, &e.HomeScore, &e.AwayScore, &scoreUpdated, (*jsonMap)(&e.Metadata), &e.CreatedAt, &e.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get event: %w", err)
	}
	if scoreUpdated.Valid {
		e.ScoreUpdatedAt = scoreUpdated.Time
	}
	return &e, nil
}

func (s *PGStore) ListMarkets(ctx context.Context, eventID, status string, page, pageSize int) ([]Market, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	var eventUUID interface{} = nil
	if eventID != "" {
		parsed, err := uuid.Parse(eventID)
		if err != nil {
			return nil, fmt.Errorf("list markets: invalid event uuid: %w", err)
		}
		eventUUID = parsed
	}

	query := `SELECT id, provider, provider_market_id, event_id, type, name, line, status, metadata, created_at, updated_at FROM markets
		WHERE ($1::uuid IS NULL OR event_id = $1::uuid)
		  AND ($2 = '' OR status = $2)
		ORDER BY name LIMIT $3 OFFSET $4`
	rows, err := s.db.QueryContext(ctx, query, eventUUID, status, pageSize, offset)
	if err != nil {
		return nil, fmt.Errorf("list markets: %w", err)
	}
	defer rows.Close()
	var out []Market
	for rows.Next() {
		var m Market
		if err := rows.Scan(&m.ID, &m.Provider, &m.ProviderMarketID, &m.EventID, &m.Type, &m.Name, &m.Line, &m.Status, (*jsonMap)(&m.Metadata), &m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, fmt.Errorf("list markets: scan: %w", err)
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list markets: rows: %w", err)
	}
	return out, nil
}

func (s *PGStore) ListOutcomes(ctx context.Context, marketID, status string, page, pageSize int) ([]Outcome, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	var marketUUID interface{} = nil
	if marketID != "" {
		parsed, err := uuid.Parse(marketID)
		if err != nil {
			return nil, fmt.Errorf("list outcomes: invalid market uuid: %w", err)
		}
		marketUUID = parsed
	}

	query := `SELECT id, provider, provider_outcome_id, market_id, name, odds, status, metadata, created_at, updated_at FROM outcomes
		WHERE ($1::uuid IS NULL OR market_id = $1::uuid)
		  AND ($2 = '' OR status = $2)
		ORDER BY name LIMIT $3 OFFSET $4`
	rows, err := s.db.QueryContext(ctx, query, marketUUID, status, pageSize, offset)
	if err != nil {
		return nil, fmt.Errorf("list outcomes: %w", err)
	}
	defer rows.Close()
	var out []Outcome
	for rows.Next() {
		var o Outcome
		if err := rows.Scan(&o.ID, &o.Provider, &o.ProviderOutcomeID, &o.MarketID, &o.Name, &o.Odds, &o.Status, (*jsonMap)(&o.Metadata), &o.CreatedAt, &o.UpdatedAt); err != nil {
			return nil, fmt.Errorf("list outcomes: scan: %w", err)
		}
		out = append(out, o)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list outcomes: rows: %w", err)
	}
	return out, nil
}

func (s *PGStore) ListLiveScores(ctx context.Context, sportID, leagueID string, page, pageSize int) ([]Event, error) {
	out, err := s.ListEvents(ctx, sportID, leagueID, "live", page, pageSize)
	if err != nil {
		return nil, fmt.Errorf("list live scores: %w", err)
	}
	return out, nil
}
