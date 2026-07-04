CREATE TABLE transactions (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id TEXT NOT NULL,
  wallet_id UUID NOT NULL REFERENCES wallets(id) ON DELETE CASCADE,
  type TEXT NOT NULL CHECK (type IN ('deposit','withdrawal','bet','win','fee','adjustment')),
  amount NUMERIC(28,8) NOT NULL,
  balance_before NUMERIC(28,8) NOT NULL,
  balance_after NUMERIC(28,8) NOT NULL,
  status TEXT NOT NULL CHECK (status IN ('pending','completed','failed')),
  reference_id TEXT UNIQUE,
  metadata JSONB,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_transactions_user_id ON transactions(user_id);
