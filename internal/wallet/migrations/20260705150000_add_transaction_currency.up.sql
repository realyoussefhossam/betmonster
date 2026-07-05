ALTER TABLE transactions ADD COLUMN currency TEXT;

CREATE INDEX idx_transactions_currency ON transactions(currency);
