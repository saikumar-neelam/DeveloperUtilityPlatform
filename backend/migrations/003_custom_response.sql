-- Custom response configuration per endpoint.
-- When a webhook hits /hook/{id}, the backend replies with these values
-- instead of the fixed 200 {"status":"received"} response.

ALTER TABLE endpoints
  ADD COLUMN IF NOT EXISTS response_status       INTEGER NOT NULL DEFAULT 200,
  ADD COLUMN IF NOT EXISTS response_content_type TEXT    NOT NULL DEFAULT 'application/json',
  ADD COLUMN IF NOT EXISTS response_headers      JSONB   NOT NULL DEFAULT '{}',
  ADD COLUMN IF NOT EXISTS response_body         TEXT    NOT NULL DEFAULT '';
