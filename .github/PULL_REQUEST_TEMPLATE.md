## What this change does

<!-- One or two sentences. -->

## Why

<!-- The problem it solves, or the issue it closes. -->

Closes #

## How

<!-- The approach, in a few lines. Mention new packages, migrations or API changes. -->

## Checklist

- [ ] `make fmt vet race lint` passes
- [ ] `cd web && npm run typecheck && npm run build` passes
- [ ] New behaviour has tests
- [ ] Migrations are added under `internal/db/migrations` and are reversible
- [ ] `docs/` is updated when behaviour or the API changes
- [ ] No secrets, tokens, keys or database files are included
