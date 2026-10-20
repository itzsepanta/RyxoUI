# RyxoUI M0 — Core: store, auth, RBAC, audit, API skeleton

> Inline execution (self, 0-100), TDD per task, fresh-context review at the end.
> Source of truth: `docs/superpowers/plans/2026-10-20-ryxoui-panel-capability-todo.md` § M0.

**Goal:** ساخت هستهٔ پنل RyxoUI: لایهٔ داده (SQLite + Postgres)، مهاجرت‌های نسخه‌دار، سرور HTTP با زنجیرهٔ middleware، احراز هویت اپراتور (session + API token + TOTP)، RBAC با scope، audit log، outbox رویداد، تولید OpenAPI، و اسکلت فرانت (React+Tailwind+shadcn) با صفحهٔ ورود و پوستهٔ برنامه.

**Baseline:** `veil` @ `f495ec0` (دایمون Veil، مستقل از این ماژول). ماژول جدید: `github.com/ryxoui/ryxoui` در `D:\Projects\test\ryxoui`.

---

## Design

- **ماژول:** `github.com/ryxoui/ryxoui` (Go 1.26). دایرکتوری `ryxoui/` کنار `veil/`. `veil` در M1 به‌عنوان یک Engine اضافه می‌شود (dependency با `replace` تا آن زمان).
- **لایهٔ داده:** `internal/db` روی `database/sql`. درایورها: `modernc.org/sqlite` (پیش‌فرض، pure-Go) و `pgx/v5/stdlib`. یک `Dialect` کوچک: `Rebind` (`?` → `$n` برای Postgres)، `Type` نگاشت‌شده (TEXT/INTEGER/BOOLEAN/TIMESTAMP)، `Now()`.
- **مهاجرت:** `internal/db/migrations/*.sql` با `go:embed`؛ هر فایل شامل بخش‌های `-- +sqlite` و `-- +postgres` (نگاشت dialect) یا یک فایل پرتابل + override. اجرا در `tx`، ثبت در `schema_migrations(version, applied_at, checksum)`. بررسی checksum برای تشخیص drift.
- **مدل داده اولیه:** `operators`, `roles`, `operator_sessions`, `api_tokens`, `settings`, `audit_log`, `event_outbox`, `event_deliveries`.
- **احراز هویت:**
  - رمز عبور اپراتور: Argon2id (`golang.org/x/crypto/argon2`) با salt تصادفی ۱۶ بایت و params (`time=1, mem=64MiB, threads=4`); ذخیره به شکل PHC string.
  - session: توکن تصادفی ۳۲ بایت، فقط SHA-256 آن ذخیره می‌شود؛ کوکی `ryxoui_session` (HttpOnly, SameSite=Strict, Path=/, Secure بر اساس TLS mode). چرخش توکن بعد از login.
  - API token: قالب `ryx_<base64url(32B)>`, ذخیرهٔ SHA-256، دارای scope list و تاریخ انقضا.
- **RBAC:** نقش‌های پایه `owner` / `operator` / `viewer` (builtin). نقش سفارشی با `base` + `scopes` (comma-separated). `internal/authz` تابع `Has(role, scope) bool` و `ScopesFor(role)`. middleware `Require(scope)` در هر handler صریح است → fail-closed.
- **audit:** `audit_log(actor_id, actor_name, action, entity, entity_id, before_json, after_json, ip, user_agent, request_id, created_at)`. نوشتن از لایهٔ سرویس (نه از handler) و همیشه در همان tx عملیات.
- **outbox:** `event_outbox(event, payload_json, created_at)` + dispatcher (goroutine) که هر ۲ ثانیه polling می‌کند؛ در M0 فقط مصرف‌کنندهٔ webhook ندارد ولی جدول و مکانیزم retry کامل است (delivery attempt، backoff، status).
- **قرارداد API:** پیشوند `/api`. پاسخ خطا: `{"error":{"code":"...","message":"..."}}`. لیست‌ها: `{"data":[...],"next_cursor":null}`. هدر نسخه: `X-RyxoUI-Api-Version: 1`. صفحه‌بندی: `?cursor=&limit=` (سقف ۵۰۰).
- **OpenAPI:** یک `internal/api/spec` که routeها را با `net/http` جدید (`GET /api/openapi/{$}`) از همان registry تولید می‌کند → `openapi.json` + `openapi.yaml`.
- **فرانت:** `ryxoui/web` با Vite + React 18 + TS + Tailwind + shadcn/ui؛ خروجی `web/dist` و embed در Go با `//go:embed web/dist`؛ در dev، Vite proxy → `http://127.0.0.1:8080`.
- **بدون** وابستگی به نمونهٔ مرجع: هیچ کد/متن/ظاهر/نامی از آن وارد نمی‌شود (خط قرمز §۰.۲ سند قابلیت).

---

## Tasks

1. **اسکلت ماژول** — `ryxoui/go.mod`، `cmd/ryxoui/main.go` (زیر‌فرمان‌های `serve|migrate|version`)، `Makefile` (build/test/lint/vet/fmt)، `.golangci.yml`، `internal/...` خالی.
   *Acceptance:* `go build ./...` سبز؛ `ryxoui version` چاپ نسخه.

2. **لایهٔ db** — `internal/db/db.go` (Open/Close/Ping، max conns)، `internal/db/dialect.go` (`Rebind`,`Type`,`Now`,`Placeholder`)، `internal/db/migrate.go` (embed + checksum + tx)، `internal/db/tx.go` (کمکی `InTx`).
   *Acceptance:* تست با SQLite in-memory: دو migration اجرا و idempotent؛ drift checksum تشخیص داده شود.

3. **مهاجرت‌های اولیه** — `internal/db/migrations/0001_init.sql` (جدول‌های بالا + ایندکس‌ها) با بخش‌های dialect.
   *Acceptance:* روی SQLite و (اختیاری) Postgres اعمال شود؛ schema با `PRAGMA table_info` بررسی شود.

4. **سرور HTTP + middleware** — `internal/api/server.go` (router، graceful shutdown، timeouts)، `internal/api/middleware.go` (request-id → log → recover → CORS/CSRF → rate-limit → authn → authz).
   *Acceptance:* تست: header `X-Request-Id` برگردد؛ panic به 500 + لاگ تبدیل شود؛ درخواست بدون auth به 401.

5. **سرویس احراز هویت** — `internal/auth/password.go` (Argon2id + verify)، `internal/auth/session.go` (issue/verify/rotate/revoke، cookie helper)، `internal/auth/apitoken.go` (generate/verify/scope).
   *Acceptance:* تست رمز (correct/wrong)، چرخش session، توکن منقضی رد شود.

6. **RBAC** — `internal/authz/scopes.go` (فهرست scopeهای M0 + اعتبارسنجی)، `internal/authz/roles.go` (ساخت نقش، inheritance پایه، `Has`)، `internal/authz/middleware.go`.
   *Acceptance:* نقش `viewer` نتواند `operators:write` را بگذراند؛ scope ناشناخته در نقش سفارشی رد شود.

7. **audit** — `internal/audit/audit.go` (ثبت + خواندن + facets) + helper `audit.Record(ctx, tx, ev)`.
   *Acceptance:* تغییر اپراتور با before/after ثبت شود؛ خواندن فقط با scope `audit:read`.

8. **outbox** — `internal/events/outbox.go` (Publish در همان tx)، `internal/events/dispatcher.go` (poll + retry/backoff + dead-letter)، `internal/events/types.go` (ثبت event typeهای M0: `operator.created`, `operator.login`, `operator.login_failed`, `role.updated`).
   *Acceptance:* رویداد منتشرشده در outbox ذخیره و توسط dispatcher به `event_deliveries` منتقل شود؛ هیچ payload secret ندارد (تست خودکار).

9. **handlerهای M0** — `internal/api/handlers_auth.go` (login/logout/me), `internal/api/handlers_operators.go`, `internal/api/handlers_roles.go`, `internal/api/handlers_settings.go`, `internal/api/handlers_audit.go`, `internal/api/handlers_scopes.go`.
   *Acceptance:* e2e — ثبت اپراتور اول (setup) → login → `/api/me` → ساخت نقش → عملیات write با نقش viewer → 403.

10. **OpenAPI generator** — `internal/api/spec/spec.go` (+ JSON/YAML emitter دستی یا `gopkg.in/yaml.v3`).
    *Acceptance:* `GET /api/openapi.json` شامل مسیرهای M0 و schema خطا؛ تست snapshot حداقلی.

11. **اسکلت فرانت** — `ryxoui/web`: Vite + React + TS + Tailwind + shadcn؛ صفحهٔ login، shell (nav/sidebar/user menu)، صفحهٔ اپراتورها، صفحهٔ نقش‌ها، صفحهٔ audit؛ i18n EN/FA با RTL.
    *Acceptance:* `npm run build` سبز؛ `go:embed` فایل‌ها را سرو می‌کند؛ login کامل کار می‌کند؛ در RTL چیدمان نشکند.

12. **setup + CLI** — `cmd/ryxoui serve` (flags: listen, db, basepath, tls mode)، مسیر `/api/setup` فقط وقتی هیچ اپراتوری نیست؛ `migrate`/`version`.
    *Acceptance:* تازه‌سازی: `serve` → `/api/setup` → ساخت owner → login.

13. **تست‌ها و CI** — تست واحد store/auth/authz/audit/events، تست integration با SQLite، e2e با `httptest`؛ GitHub Actions: build/test (`-race`)/vet/gofmt/lint/frontend build.
    *Acceptance:* `go test ./... -race` سبز؛ `go vet ./...` و `gofmt -l .` خالی.

14. **مستندات** — `ryxoui/README.md` (نصب/اجرا)، `docs/architecture.md` (لایه‌ها)، `docs/api.md` (تولیدشده از OpenAPI).
    *Acceptance:* دستور `ryxoui serve` برای تازه‌کار کافی باشد.

---

## Definition of Done (M0)

- [ ] `go build ./...`، `go test ./... -race`، `go vet ./...`، `gofmt -l .` همگی سبز
- [ ] e2e: setup → login → session cookie → `/api/me` → ساخت نقش → 403 برای نقش ناکافی → audit ثبت‌شده
- [ ] outbox: رویداد → delivery attempt → retry ثبت‌شده
- [ ] OpenAPI تولیدشده و شامل همهٔ routeهای M0
- [ ] فرانت: build سبز، login کامل، RTL سالم
- [ ] هیچ secret در لاگ/پاسخ/audit (تست خودکار)
