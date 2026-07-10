CREATE TABLE bets (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id text NOT NULL,
  event_id uuid NOT NULL,
  market_id uuid NOT NULL,
  outcome_id uuid NOT NULL,
  odds text NOT NULL,
  stake text NOT NULL,
  potential_payout text NOT NULL,
  currency text NOT NULL,
  status text NOT NULL CHECK (status IN ('pending', 'won', 'lost', 'cancelled', 'settled')),
  reference_id text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  settled_at timestamptz,
  UNIQUE(user_id, reference_id)
);

CREATE INDEX idx_bets_user_id ON bets(user_id);
CREATE INDEX idx_bets_status ON bets(status);
CREATE INDEX idx_bets_event_id ON bets(event_id);
CREATE INDEX idx_bets_outcome_id ON bets(outcome_id);
CREATE INDEX idx_bets_reference_id ON bets(reference_id);
