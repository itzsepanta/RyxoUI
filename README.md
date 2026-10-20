# RyxoUI

Self hosted, multi protocol control panel for VPN nodes.

RyxoUI is a single Go binary that owns the whole life cycle of a node:
registration over SSH, protocol engines, user accounts, quotas, statistics,
subscription portals and the management interface itself.

The panel is independent software. It does not copy code, text, appearance
or branding from any other project; the only reference used during design
was the publicly observable *capability surface* of comparable panels.

## Status

Milestone **M0** (core) is implemented:

- storage layer with SQLite and Postgres, versioned migrations and drift detection
- operator accounts with Argon2id passwords, session cookies and API tokens
- role based access control with 37 scopes across 14 groups, three builtin roles
- audit log written inside the same transaction as every change
- durable event outbox with retry, backoff and delivery records
- HTTP API with a generated OpenAPI description
- React interface with English and Persian (RTL) translations

See `docs/superpowers/plans/2026-10-20-ryxoui-panel-capability-todo.md` for
the full roadmap.

## Build

```bash
# frontend (writes into internal/web/dist)
cd web && npm install && npm run build

# backend
go build -o bin/ryxoui ./cmd/ryxoui
```

Or use the container image:

```bash
docker build -t ryxoui .
docker run -p 8080:8080 -v ryxoui-data:/var/lib/ryxoui ryxoui
```

## Run

```bash
ryxoui serve --listen 127.0.0.1:8080 --db ryxoui.db
```

Open `http://127.0.0.1:8080` and create the owner account.

### Commands

| Command | Purpose |
| --- | --- |
| `ryxoui serve` | start the panel |
| `ryxoui migrate` | apply the database schema |
| `ryxoui version` | print the version |

### Serve flags

| Flag | Default | Purpose |
| --- | --- | --- |
| `--listen` | `127.0.0.1:8080` | address to listen on |
| `--db` | `ryxoui.db` | database path or connection string |
| `--driver` | `sqlite` | `sqlite` or `postgres` |
| `--basepath` | empty | prefix for every route |
| `--tls` | `none` | `none` or `auto` |
| `--cert-file`, `--key-file` | empty | certificate for `auto` |
| `--log-level` | `info` | `debug`, `info`, `warn` or `error` |

## Auto update

The panel can update itself from the releases of its GitHub repository.

- It checks the release feed on a fixed interval (six hours by default) and
  records the result in the `update_state` table.
- The **Updates** page shows the running version, the newest release, the
  artefact that matches the platform and the release notes.
- Installing a release downloads the artefact, verifies its SHA-256
  checksum against the `.sha256` file published next to it, and refuses to
  install anything that does not match.
- The running binary is renamed instead of overwritten, so the replacement
  works on Linux and on Windows, and the previous binary is kept as
  `ryxoui.old`.
- When a systemd unit is configured the panel restarts itself after the
  install; otherwise it reports that a restart is needed.

```bash
ryxoui serve --db ryxoui.db --repo itzsepanta/RyxoUI --interval 6h --service ryxoui.service
ryxoui update --repo itzsepanta/RyxoUI            # report the newest release
ryxoui update --repo itzsepanta/RyxoUI --apply    # install it
```

Releases are built by `.github/workflows/release.yml`. Every release carries
one archive per platform, named `ryxoui_<version>_<os>_<arch>.tar.gz`, and
the matching `.sha256` file that the panel verifies before installing.

Updating the panel itself needs the `panel:update` scope, which only the
owner role holds by default.

## Development

```bash
make test        # unit and integration tests
make race        # the same tests with the race detector
make vet         # go vet
make lint        # gofmt check
make web         # rebuild the frontend
make build       # build the binary
```

Run the panel locally with the Vite dev server:

```bash
ryxoui serve --db ryxoui.db      # terminal 1
cd web && npm run dev            # terminal 2, proxies /api to :8080
```

## Layout

```
cmd/ryxoui        command line entry point
internal/db       storage, dialect and migrations
internal/auth     passwords, sessions, API tokens, TOTP
internal/authz    scopes, roles and permission checks
internal/audit    the audit log
internal/events   event catalogue, outbox and dispatcher
internal/api      HTTP API, route table and OpenAPI generation
internal/web      embedded frontend assets
web               frontend sources (React, TypeScript, Vite, Tailwind)
```

## API

The API lives under `/api` and authenticates either with the session cookie
or with an `Authorization: Bearer <token>` header.

```bash
curl -c cookies -X POST http://127.0.0.1:8080/api/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"owner","password":"your password"}'

curl -b cookies http://127.0.0.1:8080/api/operators
```

The machine readable description is served at `/api/openapi.json` and
`/api/openapi.yaml`.
