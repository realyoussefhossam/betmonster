ALTER TABLE bets
    DROP COLUMN IF EXISTS debit_transaction_id,
    DROP COLUMN IF EXISTS credit_transaction_id;

ALTER TABLE bets DROP CONSTRAINT IF EXISTS bets_status_check;
ALTER TABLE bets ADD CONSTRAINT bets_status_check CHECK (status IN ('pending', 'won', 'lost', 'cancelled', 'settled'));
