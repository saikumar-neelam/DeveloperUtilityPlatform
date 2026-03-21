-- Webhook debugging platform schema

CREATE TABLE endpoints (
    id          TEXT        PRIMARY KEY,
    name        TEXT        NOT NULL,
    target_url  TEXT        NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE webhook_requests (
    id           TEXT        PRIMARY KEY,
    endpoint_id  TEXT        NOT NULL REFERENCES endpoints(id) ON DELETE CASCADE,
    method       TEXT        NOT NULL,
    headers      JSONB       NOT NULL DEFAULT '{}',
    query_params JSONB       NOT NULL DEFAULT '{}',
    content_type TEXT        NOT NULL DEFAULT '',
    body_size    BIGINT      NOT NULL DEFAULT 0,
    s3_key       TEXT        NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_webhook_requests_endpoint_created
    ON webhook_requests(endpoint_id, created_at DESC);

CREATE TABLE replay_results (
    id            TEXT        PRIMARY KEY,
    request_id    TEXT        NOT NULL REFERENCES webhook_requests(id) ON DELETE CASCADE,
    status_code   INT         NOT NULL DEFAULT 0,
    response_body TEXT        NOT NULL DEFAULT '',
    duration_ms   BIGINT      NOT NULL DEFAULT 0,
    error         TEXT        NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_replay_results_request_id
    ON replay_results(request_id, created_at DESC);
