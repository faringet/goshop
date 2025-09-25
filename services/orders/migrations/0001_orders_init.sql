-- +goose Up
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS orders (
                                      id            UUID           PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id       UUID           NOT NULL,
    status        TEXT           NOT NULL, -- new|reserved|paid|canceled|failed
    total_amount  NUMERIC(12,2)  NOT NULL DEFAULT 0,
    currency      TEXT           NOT NULL DEFAULT 'RUB',
    created_at    TIMESTAMPTZ    NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ    NOT NULL DEFAULT now()
    );
CREATE INDEX IF NOT EXISTS idx_orders_user_id ON orders(user_id);
CREATE INDEX IF NOT EXISTS idx_orders_status  ON orders(status);

-- outbox
CREATE TABLE IF NOT EXISTS outbox (
                                      id            BIGSERIAL      PRIMARY KEY,
                                      agg_type      TEXT           NOT NULL,
                                      agg_id        UUID           NOT NULL,
                                      topic         TEXT           NOT NULL,
                                      key           BYTEA,
                                      headers       JSONB          NOT NULL DEFAULT '[]'::jsonb,
                                      payload       JSONB          NOT NULL,
                                      created_at    TIMESTAMPTZ    NOT NULL DEFAULT now(),
    available_at  TIMESTAMPTZ    NOT NULL DEFAULT now(),
    published_at  TIMESTAMPTZ,
    error         TEXT,
    retries       INT            NOT NULL DEFAULT 0
    );
CREATE INDEX IF NOT EXISTS idx_outbox_pub_null  ON outbox(published_at) WHERE published_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_outbox_available ON outbox(available_at)  WHERE published_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_outbox_agg       ON outbox(agg_type, agg_id);

-- функция должна быть внутри StatementBegin/StatementEnd
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $func$
BEGIN
  NEW.updated_at := now();
RETURN NEW;
END;
$func$;
-- +goose StatementEnd

DROP TRIGGER IF EXISTS trg_orders_updated_at ON orders;
CREATE TRIGGER trg_orders_updated_at
    BEFORE UPDATE ON orders
    FOR EACH ROW
    EXECUTE PROCEDURE set_updated_at();

-- +goose Down
DROP TRIGGER IF EXISTS trg_orders_updated_at ON orders;

-- +goose StatementBegin
DROP FUNCTION IF EXISTS set_updated_at();
-- +goose StatementEnd

DROP TABLE IF EXISTS outbox;
DROP TABLE IF EXISTS orders;
