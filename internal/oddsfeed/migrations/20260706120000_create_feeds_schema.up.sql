CREATE TABLE sports (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  provider text NOT NULL,
  provider_sport_id text NOT NULL,
  slug text NOT NULL,
  name text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(provider, provider_sport_id)
);

CREATE TABLE leagues (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  provider text NOT NULL,
  provider_league_id text NOT NULL,
  sport_id uuid NOT NULL REFERENCES sports(id) ON DELETE CASCADE,
  name text NOT NULL,
  country text,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(provider, provider_league_id)
);

CREATE TABLE events (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  provider text NOT NULL,
  provider_event_id text NOT NULL,
  league_id uuid NOT NULL REFERENCES leagues(id) ON DELETE CASCADE,
  sport_id uuid NOT NULL REFERENCES sports(id) ON DELETE CASCADE,
  home_participant text NOT NULL,
  away_participant text NOT NULL,
  starts_at timestamptz NOT NULL,
  status text NOT NULL CHECK (status IN ('upcoming', 'live', 'paused', 'finished', 'cancelled', 'postponed')),
  home_score text,
  away_score text,
  score_updated_at timestamptz,
  metadata jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(provider, provider_event_id)
);

CREATE INDEX idx_events_status ON events(status);
CREATE INDEX idx_events_starts_at ON events(starts_at);
CREATE INDEX idx_events_sport_league ON events(sport_id, league_id);

CREATE TABLE markets (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  provider text NOT NULL,
  provider_market_id text NOT NULL,
  event_id uuid NOT NULL REFERENCES events(id) ON DELETE CASCADE,
  type text NOT NULL,
  name text NOT NULL,
  line text,
  status text NOT NULL CHECK (status IN ('active', 'suspended', 'settled', 'cancelled')),
  metadata jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(provider, provider_market_id)
);

CREATE INDEX idx_markets_event_id ON markets(event_id);

CREATE TABLE outcomes (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  provider text NOT NULL,
  provider_outcome_id text NOT NULL,
  market_id uuid NOT NULL REFERENCES markets(id) ON DELETE CASCADE,
  name text NOT NULL,
  odds text NOT NULL,
  status text NOT NULL CHECK (status IN ('active', 'suspended', 'won', 'lost', 'cancelled')),
  metadata jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(provider, provider_outcome_id)
);

CREATE INDEX idx_outcomes_market_id ON outcomes(market_id);

CREATE TABLE odds_snapshots (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  event_id uuid NOT NULL REFERENCES events(id) ON DELETE CASCADE,
  market_id uuid NOT NULL REFERENCES markets(id) ON DELETE CASCADE,
  snapshot_at timestamptz NOT NULL DEFAULT now(),
  odds jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_odds_snapshots_event ON odds_snapshots(event_id, snapshot_at);
