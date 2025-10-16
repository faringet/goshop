-- +goose Up
CREATE TABLE IF NOT EXISTS payments_inbox (
                                              id           BIGSERIAL PRIMARY KEY,
                                              topic        TEXT        NOT NULL,
                                              partition    INT         NOT NULL,
                                              "offset"     BIGINT      NOT NULL,
                                              key          BYTEA,
                                              payload      JSONB       NOT NULL,
                                              received_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    processed_at TIMESTAMPTZ
    );

CREATE UNIQUE INDEX IF NOT EXISTS payments_inbox_uniq_tpo
    ON payments_inbox (topic, partition, "offset");

-- +goose Down
DROP TABLE IF EXISTS payments_inbox;
