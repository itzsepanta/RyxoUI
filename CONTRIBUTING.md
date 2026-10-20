# Contributing to RyxoUI

Thanks for helping out. This document explains how to get a change through
the door.

## Ground rules

1. The panel is **independent software**. Do not copy code, text, interface
   elements, icons or branding from other projects. The design may be
   inspired by the *capabilities* of other panels, never by their
   expression.
2. There is **no financial layer**: no payment, pricing, invoicing, coupons
   or resale. Quotas and traffic packages are fine, selling them is not.
3. Keep the Go module dependency free where possible. A new dependency needs
   a reason in the pull request description.

## Getting set up

```bash
git clone https://github.com/itzsepanta/RyxoUI.git
cd RyxoUI

cd web && npm ci && npm run build && cd ..
go build -o bin/ryxoui ./cmd/ryxoui
```

Run the backend and the frontend separately while you work:

```bash
ryxoui serve --db ryxoui.db      # terminal 1
cd web && npm run dev            # terminal 2
```

## Before you open a pull request

```bash
make fmt        # gofmt
make vet        # go vet
make race       # go test -race
make lint       # gofmt check
cd web && npm run typecheck && npm run build
```

The continuous integration runs the same checks, builds the frontend and
the binary, and builds the container image.

## Style

- **Go**: standard library first, errors wrapped with context, exported
  identifiers documented, tables and queries in `internal/db/migrations`.
- **Frontend**: TypeScript strict, functional components, Tailwind utility
  classes with the design tokens in `web/src/index.css`, no component
  library beyond the small set in `web/src/components/ui.tsx`.
- **Comments**: explain why, not what. Do not narrate the code.
- **Tests**: every behaviour that can break gets a test. Use the table
  driven style already used in the repository.

## Reporting bugs

Open an issue with the version (`ryxoui version`), the operating system, the
steps to reproduce and what you expected instead. A failing test or a
reproduction script is worth more than a long description.

## Suggesting features

Describe the problem first and the solution second. Say which milestone of
[docs/roadmap.md](docs/roadmap.md) it belongs to. Pull requests that
implement a large feature without an issue will be asked for a design
first.

## Commits and pull requests

- One topic per commit, written in the imperative mood.
- Reference the issue the commit closes.
- Update `docs/` when behaviour changes.
- Never commit secrets, certificates, private keys or database files.

## Reporting security issues

Do not open a public issue. Follow [SECURITY.md](SECURITY.md).
