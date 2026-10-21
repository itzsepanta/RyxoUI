-- The presets of the panel: ready made configurations of the protocols.

CREATE TABLE IF NOT EXISTS presets (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  kind TEXT NOT NULL,
  type TEXT NOT NULL,
  remark TEXT NOT NULL DEFAULT '',
  config TEXT NOT NULL DEFAULT '',
  created_at BIGINT NOT NULL DEFAULT 0,
  updated_at BIGINT NOT NULL DEFAULT 0
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_presets_name ON presets (name);
