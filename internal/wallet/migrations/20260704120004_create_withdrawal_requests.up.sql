CREATE TABLE withdrawal_requests (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id TEXT NOT NULL,
  wallet_id UUID NOT NULL REFERENCES wallets(id),
  amount NUMERIC(28,8) NOT NULL,
  currency TEXT NOT NULL,
  destination_address TEXT NOT NULL,
  chain TEXT NOT NULL,
  status TEXT NOT NULL CHECK (status IN ('pending','approved','rejected','completed')),
  tx_hash TEXT,
  reviewed_by TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_withdrawal_requests_status ON withdrawal_requests(status);
