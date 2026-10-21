-- The certificates of the panel itself.

CREATE TABLE IF NOT EXISTS panel_certificates (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  kind TEXT NOT NULL,
  certificate TEXT NOT NULL,
  private_key TEXT NOT NULL,
  fingerprint TEXT NOT NULL,
  created_at BIGINT NOT NULL DEFAULT 0,
  updated_at BIGINT NOT NULL DEFAULT 0
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_panel_certificates_name ON panel_certificates (name);
