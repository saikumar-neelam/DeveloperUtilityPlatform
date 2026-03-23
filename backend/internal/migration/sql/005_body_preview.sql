-- Store first 500 chars of request body for client-side search.
ALTER TABLE webhook_requests
  ADD COLUMN IF NOT EXISTS body_preview TEXT NOT NULL DEFAULT '';

-- Store notification email on endpoints.
ALTER TABLE endpoints
  ADD COLUMN IF NOT EXISTS notify_email TEXT NOT NULL DEFAULT '';
