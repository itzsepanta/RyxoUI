# RyxoUI architecture

The panel is a single Go binary with an embedded single page application.
Everything lives in one process: the HTTP API, the scheduler, the event
dispatcher and the static assets.

## Layers

```
        browser (React, TypeScript, Tailwind)
                 │  fetch /api, session cookie
                 ▼
        internal/api ── route table ── OpenAPI generator
                 │  middleware: request id, log, recover, rate limit, authn, authz
                 ▼
        domain services
        ├── internal/auth      passwords, sessions, API tokens, TOTP
        ├── internal/authz     scopes, roles, permission checks
        ├── internal/audit     audit log
        ├── internal/events    event catalogue, outbox, dispatcher
        └── internal/db        storage, dialect, migrations
                 ▼
        SQLite (modernc) or Postgres (pgx)
```

## Storage rules

The schema is portable across both engines, which keeps one migration file
for both:

- identifiers are `TEXT` and generated in Go (`internal/id`), so no engine
  specific auto increment syntax is needed;
- timestamps are `BIGINT` holding Unix seconds;
- booleans are `INTEGER` holding `0` or `1`;
- queries are written with `?` placeholders and rewritten to `$n` for
  Postgres by `db.Rebind`.

Migrations live in `internal/db/migrations` and are embedded into the
binary. Each file is applied once inside a transaction and its checksum is
stored in `schema_migrations`; editing an applied migration is reported as
drift instead of being silently ignored.

## Authentication

| Credential | Storage | Lifetime |
| --- | --- | --- |
| Password | Argon2id (PHC string) | until changed |
| Session | SHA-256 of a 32 byte random token, in an HttpOnly cookie | 12 h, or 30 d with "remember" |
| API token | SHA-256 of `ryx_<random>`, returned once at creation | until revoked |

Sessions and API tokens are resolved into a `Principal` that carries the
operator, the role and the effective scopes.

## Authorisation

Scopes are names such as `users:read`. Roles are named sets of scopes; a
custom role inherits the scopes of its base role. Three roles are built in:
`owner` (everything), `operator` (day to day work) and `viewer` (read only).

Every route in the table declares the scope it needs. The check happens in
middleware, so a handler cannot forget it, and it fails closed: a request
without a matching scope is rejected with `403`.

## Auditing

Every change writes an audit entry in the same transaction as the change
itself. Entries keep the actor, the action, the entity, the previous and the
new state, the address, the user agent and the request identifier, so the log
can never disagree with the data.

## Events

Changes publish an event to a durable outbox. A dispatcher polls the outbox,
hands each event to the subscribers and records every attempt with its
status, the error and the next retry time. Failures use an exponential
backoff and end up in a dead letter state instead of being lost.

Event types carry a schema and the scope required to read them. The
catalogue is exposed at `/api/event-types`.

## Self update

The panel can replace its own binary with a newer release.

```
release feed (GitHub API)
        │  check every 6 h
        ▼
internal/update.Manager ──► update_state table (last known state)
        │
        ├─ download the artefact for runtime.GOOS/runtime.GOARCH
        ├─ verify the SHA-256 against the published .sha256 file
        ├─ extract and rename the running binary
        └─ ask systemd to restart (optional)
```

Rules:

- a release without a checksum file is never installed;
- a download whose checksum does not match is discarded;
- the binary is renamed before the new one is put in place, so the
  replacement also works on Windows where a running image cannot be
  overwritten;
- the previous binary stays next to the new one as `ryxoui.old`;
- the endpoints are guarded by the `panel:update` scope.

## API conventions
- Base path: `/api`.
- Errors: `{"error":{"code":"...","message":"..."}}` with a matching HTTP
  status.
- Lists: `{"data":[...],"meta":{"count":N,"limit":N,"offset":N,"total":N}}`.
- Pagination: `?limit=` and `?offset=`, with a maximum page size.
- Correlation: every response carries `X-Request-Id`.
- Version: `X-RyxoUI-Api-Version` and the `info.version` field of the
  OpenAPI document.
