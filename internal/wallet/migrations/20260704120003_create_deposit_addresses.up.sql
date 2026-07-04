CREATE TABLE deposit_addresses (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id TEXT NOT NULL,
  currency TEXT NOT NULL,
  chain TEXT NOT NULL,
  address TEXT NOT NULL,
  xcash_deposit_id TEXT,
  status TEXT NOT NULL CHECK (status IN ('active','archived')),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_active_deposit_address
  ON deposit_addresses(user_id, currency, chain)
  WHERE status = 'active';
