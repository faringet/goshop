-- +goose Up
CREATE TABLE IF NOT EXISTS payments_outbox (
                                               id            BIGSERIAL   PRIMARY KEY,
                                               agg_type      TEXT        NOT NULL,
                                               agg_id        UUID        NOT NULL,
                                               topic         TEXT        NOT NULL,
                                               key           BYTEA,
                                               headers       JSONB       NOT NULL DEFAULT '[]'::jsonb,
                                               payload       JSONB       NOT NULL,
                                               created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    available_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at  TIMESTAMPTZ,
    error         TEXT,
    retries       INT         NOT NULL DEFAULT 0
    );

CREATE INDEX IF NOT EXISTS payments_outbox_pub_null
    ON payments_outbox(published_at) WHERE published_at IS NULL;

CREATE INDEX IF NOT EXISTS payments_outbox_available
    ON payments_outbox(available_at) WHERE published_at IS NULL;

CREATE INDEX IF NOT EXISTS payments_outbox_agg
    ON payments_outbox(agg_type, agg_id);

-- +goose Down
DROP TABLE IF EXISTS payments_outbox;
