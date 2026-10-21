-- The warnings of the nodes.

CREATE TABLE IF NOT EXISTS node_alerts (
  id TEXT PRIMARY KEY,
  node_id TEXT NOT NULL,
  kind TEXT NOT NULL,
  severity TEXT NOT NULL DEFAULT 'warning',
  message TEXT NOT NULL DEFAULT '',
  details TEXT NOT NULL DEFAULT '',
  raised_at BIGINT NOT NULL DEFAULT 0,
  cleared_at BIGINT NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_node_alerts_node ON node_alerts (node_id, cleared_at, raised_at);
CREATE INDEX IF NOT EXISTS idx_node_alerts_kind ON node_alerts (node_id, kind, cleared_at);
