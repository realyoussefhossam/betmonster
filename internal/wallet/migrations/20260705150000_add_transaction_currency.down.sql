DROP INDEX IF EXISTS idx_transactions_currency;
ALTER TABLE transactions DROP COLUMN IF EXISTS currency;
