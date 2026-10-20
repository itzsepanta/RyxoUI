-- Update state of the panel itself.

CREATE TABLE IF NOT EXISTS update_state (
	id TEXT PRIMARY KEY,
	current_version TEXT NOT NULL DEFAULT '',
	latest_version TEXT NOT NULL DEFAULT '',
	release_url TEXT NOT NULL DEFAULT '',
	release_notes TEXT NOT NULL DEFAULT '',
	asset_name TEXT NOT NULL DEFAULT '',
	checksum TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL DEFAULT 'idle',
	message TEXT NOT NULL DEFAULT '',
	checked_at BIGINT NOT NULL DEFAULT 0,
	applied_at BIGINT NOT NULL DEFAULT 0,
	updated_at BIGINT NOT NULL DEFAULT 0
);
