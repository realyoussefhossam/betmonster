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
	return json.Unmarshal(b, (*map[string]string)(m))
}

type PGStore struct {
	db *sql.DB
}

func NewPGStore(db *sql.DB) *PGStore { return &PGStore{db: db} }

func (s *PGStore) UpsertSport(ctx context.Context, sp Sport) (string, error) {
	var id string
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO sports (provider, provider_sport_id, slug, name)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (provider, provider_sport_id) DO UPDATE SET
			slug = EXCLUDED.slug,
			name = EXCLUDED.name,
			updated_at = now()
		RETURNING id
	`, sp.Provider, sp.ProviderSportID, sp.Slug, sp.Name).Scan(&id)
	return id, err
}

func (s *PGStore) UpsertLeague(ctx context.Context, l League) (string, error) {
	var id string
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO leagues (provider, provider_league_id, sport_id, name, country)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (provider, provider_league_id) DO UPDATE SET
			sport_id = EXCLUDED.sport_id,
			name = EXCLUDED.name,
			country = EXCLUDED.country,
			updated_at = now()
		RETURNING id
	`, l.Provider, l.ProviderLeagueID, uuid.MustParse(l.SportID), l.Name, l.Country).Scan(&id)
	return id, err
}

func (s *PGStore) UpsertEvent(ctx context.Context, e Event) (string, error) {
	var id string
	scoreUpdated := sql.NullTime{Time: e.ScoreUpdatedAt, Valid: !e.ScoreUpdatedAt.IsZero()}
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO events (provider, provider_event_id, league_id, sport_id, home_participant, away_participant, starts_at, status, home_score, away_score, score_updated_at, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
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
	`, e.Provider, e.ProviderEventID, uuid.MustParse(e.LeagueID), uuid.MustParse(e.SportID), e.HomeParticipant, e.AwayParticipant, e.StartsAt, e.Status, e.HomeScore, e.AwayScore, scoreUpdated, e.Metadata).Scan(&id)
	return id, err
}

func (s *PGStore) UpsertMarket(ctx context.Context, m Market) (string, error) {
	var id string
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO markets (provider, provider_market_id, event_id, type, name, line, status, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (provider, provider_market_id) DO UPDATE SET
			event_id = EXCLUDED.event_id,
			type = EXCLUDED.type,
			name = EXCLUDED.name,
			line = EXCLUDED.line,
			status = EXCLUDED.status,
			metadata = EXCLUDED.metadata,
			updated_at = now()
		RETURNING id
	`, m.Provider, m.ProviderMarketID, uuid.MustParse(m.EventID), m.Type, m.Name, m.Line, m.Status, m.Metadata).Scan(&id)
	return id, err
}

func (s *PGStore) UpsertOutcome(ctx context.Context, o Outcome) (string, error) {
	var id string
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO outcomes (provider, provider_outcome_id, market_id, name, odds, status, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (provider, provider_outcome_id) DO UPDATE SET
			market_id = EXCLUDED.market_id,
			name = EXCLUDED.name,
			odds = EXCLUDED.odds,
			status = EXCLUDED.status,
			metadata = EXCLUDED.metadata,
			updated_at = now()
		RETURNING id
	`, o.Provider, o.ProviderOutcomeID, uuid.MustParse(o.MarketID), o.Name, o.Odds, o.Status, o.Metadata).Scan(&id)
	return id, err
}

func (s *PGStore) ListSports(ctx context.Context, page, pageSize int) ([]Sport, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	rows, err := s.db.QueryContext(ctx, `SELECT id, provider, provider_sport_id, slug, name, created_at, updated_at FROM sports ORDER BY name LIMIT $1 OFFSET $2`, pageSize, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Sport
	for rows.Next() {
		var sp Sport
		if err := rows.Scan(&sp.ID, &sp.Provider, &sp.ProviderSportID, &sp.Slug, &sp.Name, &sp.CreatedAt, &sp.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, sp)
	}
	return out, rows.Err()
}

func (s *PGStore) ListLeagues(ctx context.Context, sportID string, page, pageSize int) ([]League, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	var rows *sql.Rows
	var err error
	if sportID != "" {
		rows, err = s.db.QueryContext(ctx, `SELECT id, provider, provider_league_id, sport_id, name, country, created_at, updated_at FROM leagues WHERE sport_id = $1 ORDER BY name LIMIT $2 OFFSET $3`, uuid.MustParse(sportID), pageSize, offset)
	} else {
		rows, err = s.db.QueryContext(ctx, `SELECT id, provider, provider_league_id, sport_id, name, country, created_at, updated_at FROM leagues ORDER BY name LIMIT $1 OFFSET $2`, pageSize, offset)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []League
	for rows.Next() {
		var l League
		if err := rows.Scan(&l.ID, &l.Provider, &l.ProviderLeagueID, &l.SportID, &l.Name, &l.Country, &l.CreatedAt, &l.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

func (s *PGStore) ListEvents(ctx context.Context, sportID, leagueID, status string, page, pageSize int) ([]Event, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	query := `SELECT id, provider, provider_event_id, league_id, sport_id, home_participant, away_participant, starts_at, status, home_score, away_score, score_updated_at, metadata, created_at, updated_at FROM events WHERE 1=1`
	args := []interface{}{}
	argCount := 0
	if sportID != "" {
		argCount++
		args = append(args, uuid.MustParse(sportID))
		query += fmt.Sprintf(" AND sport_id = $%d", argCount)
	}
	if leagueID != "" {
		argCount++
		args = append(args, uuid.MustParse(leagueID))
		query += fmt.Sprintf(" AND league_id = $%d", argCount)
	}
	if status != "" {
		argCount++
		args = append(args, status)
		query += fmt.Sprintf(" AND status = $%d", argCount)
	}
	argCount++
	args = append(args, pageSize)
	query += fmt.Sprintf(" ORDER BY starts_at DESC LIMIT $%d", argCount)
	argCount++
	args = append(args, offset)
	query += fmt.Sprintf(" OFFSET $%d", argCount)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Event
	for rows.Next() {
		var e Event
		var scoreUpdated sql.NullTime
		if err := rows.Scan(&e.ID, &e.Provider, &e.ProviderEventID, &e.LeagueID, &e.SportID, &e.HomeParticipant, &e.AwayParticipant, &e.StartsAt, &e.Status, &e.HomeScore, &e.AwayScore, &scoreUpdated, (*jsonMap)(&e.Metadata), &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, err
		}
		if scoreUpdated.Valid {
			e.ScoreUpdatedAt = scoreUpdated.Time
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *PGStore) GetEvent(ctx context.Context, id string) (*Event, error) {
	var e Event
	var scoreUpdated sql.NullTime
	err := s.db.QueryRowContext(ctx, `SELECT id, provider, provider_event_id, league_id, sport_id, home_participant, away_participant, starts_at, status, home_score, away_score, score_updated_at, metadata, created_at, updated_at FROM events WHERE id = $1`, uuid.MustParse(id)).Scan(
		&e.ID, &e.Provider, &e.ProviderEventID, &e.LeagueID, &e.SportID, &e.HomeParticipant, &e.AwayParticipant, &e.StartsAt, &e.Status, &e.HomeScore, &e.AwayScore, &scoreUpdated, (*jsonMap)(&e.Metadata), &e.CreatedAt, &e.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if scoreUpdated.Valid {
		e.ScoreUpdatedAt = scoreUpdated.Time
	}
	return &e, nil
}

func (s *PGStore) ListMarkets(ctx context.Context, eventID, status string, page, pageSize int) ([]Market, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	query := `SELECT id, provider, provider_market_id, event_id, type, name, line, status, metadata, created_at, updated_at FROM markets WHERE event_id = $1`
	args := []interface{}{uuid.MustParse(eventID)}
	if status != "" {
		args = append(args, status)
		query += fmt.Sprintf(" AND status = $%d", len(args))
	}
	args = append(args, pageSize, offset)
	query += fmt.Sprintf(" ORDER BY name LIMIT $%d OFFSET $%d", len(args)-1, len(args))
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Market
	for rows.Next() {
		var m Market
		if err := rows.Scan(&m.ID, &m.Provider, &m.ProviderMarketID, &m.EventID, &m.Type, &m.Name, &m.Line, &m.Status, (*jsonMap)(&m.Metadata), &m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *PGStore) ListOutcomes(ctx context.Context, marketID, status string, page, pageSize int) ([]Outcome, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	query := `SELECT id, provider, provider_outcome_id, market_id, name, odds, status, metadata, created_at, updated_at FROM outcomes WHERE market_id = $1`
	args := []interface{}{uuid.MustParse(marketID)}
	if status != "" {
		args = append(args, status)
		query += fmt.Sprintf(" AND status = $%d", len(args))
	}
	args = append(args, pageSize, offset)
	query += fmt.Sprintf(" ORDER BY name LIMIT $%d OFFSET $%d", len(args)-1, len(args))
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Outcome
	for rows.Next() {
		var o Outcome
		if err := rows.Scan(&o.ID, &o.Provider, &o.ProviderOutcomeID, &o.MarketID, &o.Name, &o.Odds, &o.Status, (*jsonMap)(&o.Metadata), &o.CreatedAt, &o.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

func (s *PGStore) ListLiveScores(ctx context.Context, sportID, leagueID string, page, pageSize int) ([]Event, error) {
	return s.ListEvents(ctx, sportID, leagueID, "live", page, pageSize)
}
