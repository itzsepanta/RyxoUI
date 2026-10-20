-- Pending second factor challenges.

CREATE TABLE IF NOT EXISTS mfa_pending (
	id TEXT PRIMARY KEY,
	operator_id TEXT NOT NULL,
	token_hash TEXT NOT NULL,
	remember INTEGER NOT NULL DEFAULT 0,
	attempts INTEGER NOT NULL DEFAULT 0,
	created_at BIGINT NOT NULL DEFAULT 0,
	expires_at BIGINT NOT NULL DEFAULT 0
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_mfa_pending_token ON mfa_pending (token_hash);
CREATE INDEX IF NOT EXISTS idx_mfa_pending_operator ON mfa_pending (operator_id);
CREATE INDEX IF NOT EXISTS idx_mfa_pending_expiry ON mfa_pending (expires_at);
