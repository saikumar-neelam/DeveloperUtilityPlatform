-- Add expiry support for 24-hour free-tier endpoints.
ALTER TABLE endpoints ADD COLUMN IF NOT EXISTS expires_at TIMESTAMPTZ;

-- Back-fill existing endpoints so they expire 24h from now.
UPDATE endpoints SET expires_at = NOW() + INTERVAL '24 hours' WHERE expires_at IS NULL;

-- Index for the cleanup worker query.
CREATE INDEX IF NOT EXISTS idx_endpoints_expires_at ON endpoints(expires_at);
