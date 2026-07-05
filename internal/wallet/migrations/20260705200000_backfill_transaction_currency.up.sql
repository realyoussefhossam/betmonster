UPDATE transactions
SET currency = wallets.currency
FROM wallets
WHERE transactions.wallet_id = wallets.id
  AND transactions.currency IS NULL;

-- Fallback for any rows that still lack a wallet link.
UPDATE transactions
SET currency = 'UNKNOWN'
WHERE currency IS NULL;

ALTER TABLE transactions
    ALTER COLUMN currency SET DEFAULT 'UNKNOWN',
    ALTER COLUMN currency SET NOT NULL;
