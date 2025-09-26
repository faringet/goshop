-- +goose Up
CREATE TABLE IF NOT EXISTS payments (
                                        id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id     UUID        NOT NULL,
    user_id      UUID        NOT NULL,
    amount_cents BIGINT      NOT NULL CHECK (amount_cents > 0),
    currency     TEXT        NOT NULL DEFAULT 'RUB',
    status       TEXT        NOT NULL,
    provider     TEXT        NOT NULL DEFAULT 'mockpay',
    reason       TEXT,                 
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
    );

CREATE INDEX IF NOT EXISTS idx_payments_order ON payments(order_id);
CREATE INDEX IF NOT EXISTS idx_payments_user  ON payments(user_id);

-- +goose Down
DROP TABLE IF EXISTS payments;
