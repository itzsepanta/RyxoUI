-- Initial RyxoUI schema.
--
-- Portability rules: identifiers are TEXT and generated in Go, timestamps are
-- BIGINT Unix seconds, booleans are INTEGER 0/1. The file therefore runs
-- unchanged on SQLite and Postgres.

CREATE TABLE IF NOT EXISTS roles (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	base TEXT NOT NULL DEFAULT 'operator',
	builtin INTEGER NOT NULL DEFAULT 0,
	description TEXT NOT NULL DEFAULT '',
	scopes TEXT NOT NULL DEFAULT '',
	user_limit INTEGER NOT NULL DEFAULT 0,
	max_device_limit INTEGER NOT NULL DEFAULT 0,
	max_duration INTEGER NOT NULL DEFAULT 0,
	created_at BIGINT NOT NULL DEFAULT 0,
	updated_at BIGINT NOT NULL DEFAULT 0
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_roles_name ON roles (name);

CREATE TABLE IF NOT EXISTS operators (
	id TEXT PRIMARY KEY,
	username TEXT NOT NULL,
	password_hash TEXT NOT NULL,
	role_id TEXT NOT NULL,
	is_owner INTEGER NOT NULL DEFAULT 0,
	status TEXT NOT NULL DEFAULT 'active',
	last_login_at BIGINT NOT NULL DEFAULT 0,
	last_login_ip TEXT NOT NULL DEFAULT '',
	totp_secret TEXT NOT NULL DEFAULT '',
	totp_enabled_at BIGINT NOT NULL DEFAULT 0,
	totp_last_used_at BIGINT NOT NULL DEFAULT 0,
	recovery_codes TEXT NOT NULL DEFAULT '',
	created_at BIGINT NOT NULL DEFAULT 0,
	updated_at BIGINT NOT NULL DEFAULT 0
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_operators_username ON operators (username);
CREATE INDEX IF NOT EXISTS idx_operators_role ON operators (role_id);

CREATE TABLE IF NOT EXISTS operator_sessions (
	id TEXT PRIMARY KEY,
	operator_id TEXT NOT NULL,
	token_hash TEXT NOT NULL,
	ip TEXT NOT NULL DEFAULT '',
	user_agent TEXT NOT NULL DEFAULT '',
	remember INTEGER NOT NULL DEFAULT 0,
	created_at BIGINT NOT NULL DEFAULT 0,
	last_seen_at BIGINT NOT NULL DEFAULT 0,
	expires_at BIGINT NOT NULL DEFAULT 0
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_operator_sessions_token ON operator_sessions (token_hash);
CREATE INDEX IF NOT EXISTS idx_operator_sessions_operator ON operator_sessions (operator_id);
CREATE INDEX IF NOT EXISTS idx_operator_sessions_expiry ON operator_sessions (expires_at);

CREATE TABLE IF NOT EXISTS api_tokens (
	id TEXT PRIMARY KEY,
	operator_id TEXT NOT NULL,
	name TEXT NOT NULL DEFAULT '',
	token_hash TEXT NOT NULL,
	scopes TEXT NOT NULL DEFAULT '',
	rate_limit INTEGER NOT NULL DEFAULT 0,
	last_used_at BIGINT NOT NULL DEFAULT 0,
	last_used_ip TEXT NOT NULL DEFAULT '',
	expires_at BIGINT NOT NULL DEFAULT 0,
	created_at BIGINT NOT NULL DEFAULT 0
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_api_tokens_hash ON api_tokens (token_hash);
CREATE INDEX IF NOT EXISTS idx_api_tokens_operator ON api_tokens (operator_id);

CREATE TABLE IF NOT EXISTS login_attempts (
	id TEXT PRIMARY KEY,
	username TEXT NOT NULL,
	ip TEXT NOT NULL DEFAULT '',
	successful INTEGER NOT NULL DEFAULT 0,
	created_at BIGINT NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_login_attempts_username ON login_attempts (username, created_at);

CREATE TABLE IF NOT EXISTS settings (
	key TEXT PRIMARY KEY,
	value TEXT NOT NULL DEFAULT '',
	updated_at BIGINT NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS audit_log (
	id TEXT PRIMARY KEY,
	actor_id TEXT NOT NULL DEFAULT '',
	actor_name TEXT NOT NULL DEFAULT '',
	action TEXT NOT NULL,
	entity TEXT NOT NULL DEFAULT '',
	entity_id TEXT NOT NULL DEFAULT '',
	before_json TEXT NOT NULL DEFAULT '',
	after_json TEXT NOT NULL DEFAULT '',
	ip TEXT NOT NULL DEFAULT '',
	user_agent TEXT NOT NULL DEFAULT '',
	request_id TEXT NOT NULL DEFAULT '',
	created_at BIGINT NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_audit_log_created ON audit_log (created_at);
CREATE INDEX IF NOT EXISTS idx_audit_log_actor ON audit_log (actor_id, created_at);
CREATE INDEX IF NOT EXISTS idx_audit_log_entity ON audit_log (entity, entity_id);

CREATE TABLE IF NOT EXISTS event_types (
	name TEXT PRIMARY KEY,
	family TEXT NOT NULL,
	scope TEXT NOT NULL DEFAULT '',
	description TEXT NOT NULL DEFAULT '',
	payload TEXT NOT NULL DEFAULT '',
	created_at BIGINT NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS event_outbox (
	id TEXT PRIMARY KEY,
	event TEXT NOT NULL,
	payload TEXT NOT NULL DEFAULT '',
	created_at BIGINT NOT NULL DEFAULT 0,
	available_at BIGINT NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_event_outbox_event ON event_outbox (event);
CREATE INDEX IF NOT EXISTS idx_event_outbox_available ON event_outbox (available_at);

CREATE TABLE IF NOT EXISTS event_deliveries (
	id TEXT PRIMARY KEY,
	outbox_id TEXT NOT NULL,
	subscriber TEXT NOT NULL DEFAULT '',
	attempt INTEGER NOT NULL DEFAULT 0,
	status TEXT NOT NULL DEFAULT 'pending',
	status_code INTEGER NOT NULL DEFAULT 0,
	error TEXT NOT NULL DEFAULT '',
	next_at BIGINT NOT NULL DEFAULT 0,
	delivered_at BIGINT NOT NULL DEFAULT 0,
	created_at BIGINT NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_event_deliveries_outbox ON event_deliveries (outbox_id);
CREATE INDEX IF NOT EXISTS idx_event_deliveries_due ON event_deliveries (status, next_at);
