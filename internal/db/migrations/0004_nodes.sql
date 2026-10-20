-- Nodes, their keys, their health and the configuration that is applied to
-- them.

CREATE TABLE IF NOT EXISTS ssh_keys (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	public_key TEXT NOT NULL,
	private_key TEXT NOT NULL,
	comment TEXT NOT NULL DEFAULT '',
	created_at BIGINT NOT NULL DEFAULT 0
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_ssh_keys_name ON ssh_keys (name);

CREATE TABLE IF NOT EXISTS nodes (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	address TEXT NOT NULL,
	port INTEGER NOT NULL DEFAULT 62050,
	status TEXT NOT NULL DEFAULT 'connecting',
	message TEXT NOT NULL DEFAULT '',
	remark TEXT NOT NULL DEFAULT '',
	tags TEXT NOT NULL DEFAULT '',
	arch TEXT NOT NULL DEFAULT '',
	os TEXT NOT NULL DEFAULT '',
	core_version TEXT NOT NULL DEFAULT '',
	node_version TEXT NOT NULL DEFAULT '',
	multiplier REAL NOT NULL DEFAULT 1,
	enabled INTEGER NOT NULL DEFAULT 1,
	limit_bytes BIGINT NOT NULL DEFAULT 0,
	limit_period TEXT NOT NULL DEFAULT '',
	limit_start_day INTEGER NOT NULL DEFAULT 1,
	limit_start_month INTEGER NOT NULL DEFAULT 1,
	limit_disabled INTEGER NOT NULL DEFAULT 0,
	template_id TEXT NOT NULL DEFAULT '',
	ssh_host TEXT NOT NULL DEFAULT '',
	ssh_port INTEGER NOT NULL DEFAULT 22,
	ssh_user TEXT NOT NULL DEFAULT 'root',
	ssh_key_id TEXT NOT NULL DEFAULT '',
	host_key TEXT NOT NULL DEFAULT '',
	key_installed INTEGER NOT NULL DEFAULT 0,
	last_seen_at BIGINT NOT NULL DEFAULT 0,
	last_status_change BIGINT NOT NULL DEFAULT 0,
	created_at BIGINT NOT NULL DEFAULT 0,
	updated_at BIGINT NOT NULL DEFAULT 0
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_nodes_name ON nodes (name);
CREATE INDEX IF NOT EXISTS idx_nodes_status ON nodes (status);

CREATE TABLE IF NOT EXISTS node_health_samples (
	id TEXT PRIMARY KEY,
	node_id TEXT NOT NULL,
	disk_total BIGINT NOT NULL DEFAULT 0,
	disk_used BIGINT NOT NULL DEFAULT 0,
	mem_total BIGINT NOT NULL DEFAULT 0,
	mem_used BIGINT NOT NULL DEFAULT 0,
	load1 REAL NOT NULL DEFAULT 0,
	sampled_at BIGINT NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_node_health_samples_node ON node_health_samples (node_id, sampled_at);

CREATE TABLE IF NOT EXISTS node_usage (
	id TEXT PRIMARY KEY,
	node_id TEXT NOT NULL,
	date_time BIGINT NOT NULL DEFAULT 0,
	up BIGINT NOT NULL DEFAULT 0,
	down BIGINT NOT NULL DEFAULT 0
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_node_usage_node_time ON node_usage (node_id, date_time);

CREATE TABLE IF NOT EXISTS node_templates (
	node_id TEXT NOT NULL,
	template_id TEXT NOT NULL,
	PRIMARY KEY (node_id, template_id)
);

CREATE TABLE IF NOT EXISTS templates (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	remark TEXT NOT NULL DEFAULT '',
	route TEXT NOT NULL DEFAULT '',
	core TEXT NOT NULL DEFAULT '',
	created_at BIGINT NOT NULL DEFAULT 0,
	updated_at BIGINT NOT NULL DEFAULT 0
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_templates_name ON templates (name);

CREATE TABLE IF NOT EXISTS entrypoints (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	tag TEXT NOT NULL,
	type TEXT NOT NULL,
	config TEXT NOT NULL DEFAULT '',
	node_id TEXT NOT NULL DEFAULT '',
	enabled INTEGER NOT NULL DEFAULT 1,
	created_at BIGINT NOT NULL DEFAULT 0,
	updated_at BIGINT NOT NULL DEFAULT 0
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_entrypoints_tag ON entrypoints (tag);

CREATE TABLE IF NOT EXISTS egresses (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	tag TEXT NOT NULL,
	type TEXT NOT NULL,
	config TEXT NOT NULL DEFAULT '',
	node_id TEXT NOT NULL DEFAULT '',
	enabled INTEGER NOT NULL DEFAULT 1,
	created_at BIGINT NOT NULL DEFAULT 0,
	updated_at BIGINT NOT NULL DEFAULT 0
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_egresses_tag ON egresses (tag);

CREATE TABLE IF NOT EXISTS endpoints (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	node_id TEXT NOT NULL DEFAULT '',
	type TEXT NOT NULL,
	config TEXT NOT NULL DEFAULT '',
	created_at BIGINT NOT NULL DEFAULT 0,
	updated_at BIGINT NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_endpoints_node ON endpoints (node_id);

CREATE TABLE IF NOT EXISTS install_jobs (
	id TEXT PRIMARY KEY,
	node_id TEXT NOT NULL,
	status TEXT NOT NULL DEFAULT 'queued',
	step TEXT NOT NULL DEFAULT '',
	progress INTEGER NOT NULL DEFAULT 0,
	log TEXT NOT NULL DEFAULT '',
	error TEXT NOT NULL DEFAULT '',
	started_at BIGINT NOT NULL DEFAULT 0,
	finished_at BIGINT NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_install_jobs_node ON install_jobs (node_id, started_at);
