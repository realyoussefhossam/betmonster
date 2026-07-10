ALTER TABLE bets
    ADD COLUMN debit_transaction_id text,
    ADD COLUMN credit_transaction_id text;

ALTER TABLE bets DROP CONSTRAINT IF EXISTS bets_status_check;
ALTER TABLE bets ADD CONSTRAINT bets_status_check CHECK (status IN ('debit_pending', 'pending', 'won', 'lost', 'cancelled', 'settled'));
