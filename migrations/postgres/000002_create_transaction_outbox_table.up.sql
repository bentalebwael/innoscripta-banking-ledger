CREATE TABLE IF NOT EXISTS transaction_outbox (
    id BIGSERIAL PRIMARY KEY,
    transaction_id UUID NOT NULL UNIQUE,
    account_id UUID NOT NULL,
    payload JSONB NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'PENDING',
    attempts INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_attempt_at TIMESTAMPTZ,
    CONSTRAINT fk_account
        FOREIGN KEY(account_id)
        REFERENCES accounts(id)
);

CREATE INDEX idx_transaction_outbox_status ON transaction_outbox(status);
CREATE INDEX idx_transaction_outbox_account_id ON transaction_outbox(account_id);
CREATE INDEX idx_transaction_outbox_created_at ON transaction_outbox(created_at);