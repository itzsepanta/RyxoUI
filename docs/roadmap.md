# RyxoUI — سند فازبندی و TODO پنل (رقابت قابلیتی)

**تاریخ:** ۲۰۲۶-۱۰-۲۰
**وضعیت:** سند برنامهٔ اجرایی — مبنای milestoneهای M0…M6 و R
**پیش‌نیاز:** `docs/superpowers/specs/2026-10-19-ryxoui-veil-protocol-design.md` (پروتکل Veil، V0–V5 تمام‌شده)

---

## ۰. چارچوب

### ۰.۱ هدف
ساخت **پنل کنترل VPN متن‌باز، خودمیزبان و چندپروتکل** برای RyxoUI که:

- کل چرخهٔ «نود ← SSH ← موتور پروتکل ← کاربر/سهمیه ← پورتال/اشتراک» را مستقل پیاده‌سازی کند.
- از نظر **دامنهٔ قابلیت** با پنل مرجع هم‌تراز یا جلوتر باشد (نه از نظر ظاهر، متن، یا کد).
- برای ۱۰۰ تا ۵٬۰۰۰ کاربر/نصب مقیاس‌پذیر باشد.

### ۰.۲ خطوط قرمز (بدون مذاکره)
- ❌ دور زدن یا حذف لایسنس/احراز هویت پنل مرجع.
- ❌ کپی کد، متن، ظاهر، لوگو، نام‌گذاری تجاری یا دارایی بصری پنل مرجع.
- ❌ لایهٔ مالی (پرداخت، قیمت، فاکتور، کوپن، فروش مجدد/resale) در RyxoUI.
- ✅ استفادهٔ **فقط‌خواندنی** از نمونهٔ مرجع برای فهم *دامنهٔ قابلیت* و *معناشناسی API*؛ پیاده‌سازی از صفر با ادبیات و مدل دادهٔ مستقل.

### ۰.۳ روش کار
1. **Capability discovery** — استخراج: مسیرهای API، مدل داده، رویدادها، نقش‌ها، تنظیمات، قالب‌های پاسخ (خواندنی).
2. **Gap mapping** — نگاشت هر قابلیت به milestone موجود RyxoUI و تعیین «موجود / ناقص / غایب».
3. **Design independent** — تعریف مدل داده، naming، scopeها و قرارداد API **مستقل** (نه mirror).
4. **Implement by milestone** — هر milestone با ledger پیشرفت و review.

---

## ۱. شواهد استخراج‌شده از نمونهٔ مرجع

| کشف | عدد | مرجع فایل |
| --- | --- | --- |
| مسیرهای یکتای API | ۷۴۰ لیترال `/api/...` | `extracted/nexora-panel/nexora-routes.txt` |
| سند OpenAPI رسمی | ۲۱۹ مسیر / ۲۷۴ عملیات / ۱۲۳ اسکیما | `extracted/nexora-panel/openapi.json` |
| مدل داده (SQLite) | ۴۹ جدول + ایندکس‌ها | `extracted/nexora-panel/nexora-schema.txt` |
| کاتالوگ رویداد | ۵۲ رویداد در ۴ خانواده | `extracted/nexora-panel/event-summary.txt` |
| نقش‌های داخلی | ۳ نقش (سطوح دسترسی ۲۹ / ۱۶ / ۵) | `extracted/nexora-panel/roles.json` |
| کلیدهای تنظیمات | ۳۰ کلید پیکربندی | `extracted/nexora-panel/nexora-config.txt` |
| تماس‌های API از فرانت | ۱۹۷ مسیر مصرف‌شده توسط UI | `extracted/nexora-panel/fe-endpoints.txt` |
| استک فرانت | Vue 3 + Router + Pinia + vue-i18n (Vite) | `extracted/nexora-panel/fe_probe2.sh` |
| فرانت‌اندهای غیر-API | `/metrics`، `/sub/{token}`، `/install-node.sh` | `extracted/nexora-panel/openapi-summary.txt` |

**نتیجه‌گیری کلیدی:** دامنهٔ مرجع از «پنل کاربران/نودها» فراتر رفته و شامل **زیرساخت** است: مدیریت افزونه (addon) با امضای دیجیتال، نصب SSH روی هاست، ACME، tunel‌های reverse، mTLS داخلی، بکاپ رمز‌شده، و کاتالوگ رویداد قابل‌اشتراک‌گذاری. این‌ها دقیقاً همان جاهایی هستند که RyxoUI می‌تواند با طراحی مستقل، **برتری** بسازد (تنظیم‌پذیری، self-hosting بدون وابستگی، و UX فارسی/RTL).

---

## ۲. ماتریس قابلیت (خلاصهٔ نگاشت)

| حوزه | قابلیت مرجع | فاز RyxoUI |
| --- | --- | --- |
| احراز هویت | رمز عبور، TOTP + کد بازیابی، OIDC/SSO، session مدیریت‌شده، allowlist | M0 / M4 |
| RBAC | نقش‌های داخلی + نقش سفارشی با scope | M0 / M5 |
| اپراتورها | چند اپراتور، مالک، محدودیت کاربر/مدت/دستگاه | M0 |
| نودها | ثبت، SSH نصب، health، نسخه، routing، template، limit ترافیک | M1 |
| نصب از راه دور | job جریانی + probe + نسخه‌ها + cancel | M1 |
| موتور/کانفیگ | inbounds/outbounds/endpoints، presets، فرمت‌های چندگانه | M1 / M3 |
| کاربران | CRUD، bulk، generate، export، summary، renew | M2 |
| سهمیه/پلن | volume، روز، speed/IP/device limit، reset خودکار | M2 |
| آمار | usage نود/کاربر، آمار سری زمانی، گزارش top | M2 |
| قالب‌ها | template نود/کاربر/پلن، انتساب چندگانه | M2 |
| پورتال | صفحهٔ اشتراک HTML + QR + لینک‌ها + theme | M2 |
| اشتراک | `/sub/{token}` با چند فرمت (URI/QR/clash/sing-box) | M2 |
| اتصال زنده | connections زندهٔ نود/کاربر، close | M4 |
| اعلان | webhook + ایمیل + تلگرام + outbox + retry | M4 |
| بکاپ | zip رمزدار، schedule، retention، restore | M4 |
| گواهی | ACME، renew، panel/node، mTLS داخلی | M5 / R |
| پوشش دامنه | domain fronting / ingress camouflage | M5 |
| کاتالوگ رویداد | ۵۲ رویداد با schema payload + scope | M4 |
| افزونه‌ها | directory، امضا، install، health، lifecycle | R |
| تونل | reverse tunnel، balance، workers، status | R |
| rule-set | منبع بیرونی، refresh، content hash | R |
| متریک | `/metrics` Prometheus | M5 |
| OpenAPI | تولید خودکار از routeها | M5 |
| ابزارها | keypair، reality، ECH، encryption، preview نام لینک | M5 |
| self-update | check/download/apply + node cache | R |
| نصب‌کننده | اسکریپت curl + CA + باینری | M1 |

---

## ۳. تصمیم‌های معماری RyxoUI (مستقل از مرجع)

### ۳.۱ اصطلاح‌شناسی (برای مستقل‌بودن و چندمحوری‌بودن)
| مرجع | RyxoUI | دلیل |
| --- | --- | --- |
| `inbounds` | `entrypoints` | چندمحوری؛ `inbound` واژهٔ xray است |
| `outbounds` | `egresses` | همان |
| `domain_fronts` | `masks` | «پوشش دامنه» مستقل |
| `plans` | `bundles` | بدون لایهٔ مالی؛ «بستهٔ ترافیک» |
| `addons` | `extensions` | «افزونه» |
| `changes` (audit) | `audit_log` | شفاف |
| `admins` | `operators` | «اپراتور پنل» |
| `rule_sets` | `rule_feeds` | «منبع قواعد» |

> نام‌گذاری صرفاً *معناشناسی* است؛ فرمت‌ها، ساختار پیام و ظاهر، مستقل طراحی می‌شوند.

### ۳.۲ ماژول‌های Go (مونولیت)
```
cmd/ryxoui              → CLI (serve, migrate, backup, version)
internal/
  store/                → SQLite(modernc) + Postgres(pgx) + migrate + tx
  domain/               → مدل‌های دامنه + قوانین
  auth/                 → session، TOTP، OIDC، scopes، API token
  audit/                → audit_log + outbox
  nodes/                → registry، SSH، health، limits، routing
  engines/              → xray، singbox، wireguard، openvpn، mtproto، ssh، mieru، trusttunnel، sudoku، veil
  users/                → accounts، bundles، quota، reset
  traffic/              → accounting، stats، reports
  portal/               → subscription page، themes، QR، formats
  notify/               → webhook، email، telegram، outbox/retry
  backup/               → archive، schedule، restore
  certs/                → ACME، renew، mTLS
  extensions/           → registry، signature، lifecycle، jobs
  tunnels/              → reverse/relay
  api/                  → router، middleware، OpenAPI generator
  web/                  → embed SPA
```

### ۳.۳ مدل داده (جداول مستقل RyxoUI)
- **هویت:** `operators`, `roles`, `operator_sessions`, `operator_identities` (OIDC), `api_tokens`, `login_attempts`.
- **نود:** `nodes`, `node_ssh_keys`, `node_credentials` (رمز‌شده), `node_health_samples`, `node_usage`, `node_usage_by_user`, `node_install_jobs`.
- **پیکربندی:** `entrypoints`, `egresses`, `endpoints`, `templates`, `template_bindings`, `rule_feeds`, `masks`, `mask_bindings`.
- **کاربر:** `users`, `user_devices`, `bundles`, `user_bundles`, `user_limits`, `quota_ledger`.
- **اشتراک:** `subscriptions` (token هش‌شده), `subscription_clients`, `sub_themes`.
- **پایش:** `audit_log`, `event_types`, `event_outbox`, `event_deliveries`, `notifications`.
- **زیرساخت پنل:** `settings`, `certificates`, `panel_certificates`, `acme_accounts`, `mtls_certs`, `tunnels`, `tunnel_origins`, `extensions`, `extension_installs`, `backups`, `service_states`.

### ۳.۴ نقش‌ها و scopeها (طراحی مستقل)
نقش‌های پایه: `owner` / `operator` / `viewer` (+ نقش سفارشی).

| گروه scope | مقادیر |
| --- | --- |
| اپراتورها | `operators:read`, `operators:write` |
| نودها | `nodes:read`, `nodes:write`, `nodes:install` |
| کاربران | `users:read`, `users:write` |
| بسته‌ها/قالب‌ها | `bundles:read`, `bundles:write`, `templates:read`, `templates:write` |
| پیکربندی شبکه | `entrypoints:write`, `egresses:write`, `endpoints:write`, `masks:write` |
| پایش | `stats:read`, `audit:read`, `logs:read` |
| گواهی | `certs:read`, `certs:write` |
| تنظیمات | `settings:read`, `settings:write` |
| امنیت | `security:read`, `security:write`, `bans:write` |
| اعلان | `notify:read`, `notify:write` |
| یکپارچه‌سازی | `integrations:read`, `integrations:write` |
| پشتیبان | `backups:read`, `backups:write`, `backups:restore` |
| تونل/افزونه | `tunnels:read`, `tunnels:write`, `extensions:read`, `extensions:write` |
| توکن | `tokens:write` |

قواعد: `owner` همه‌چیز؛ `viewer` فقط `*:read`؛ scopeها به‌صورت نقطه‌ای‌جداشده ذخیره می‌شوند؛ هر handler با decorator صریح scope لازم را اعلام می‌کند (fail-closed).

### ۳.۵ مدل رویداد (مستقل)
خانواده‌ها: `panel`, `operator`, `node`, `user`, `traffic`, `integration`.

قواعد طراحی:
1. هر رویداد یک **schema payload** دارد (نام فیلد + نوع) که در `/api/event-types` قابل‌کشف است.
2. هر رویداد یک **scope خواندن** دارد → فقط مشترکِ دارای آن scope می‌تواند دریافت کند.
3. هیچ رویدادی **هرگز** credential/token/کلید خصوصی حمل نمی‌کند (قاعدهٔ تستی خودکار).
4. تحویل: outbox → retry با backoff → dead-letter → replay دستی.
5. امضای HMAC روی body برای webhook.

### ۳.۶ قرارداد API
- پیشوند: `/api/v1/...` (نسخه‌بندی صریح) — یا `/api/...` + هدر `X-Api-Version` (انتخاب در M0).
- پوشش (envelope) مشترک: `{ "data": [...], "page": {...} }` برای لیست‌ها؛ خطا: `{ "error": { "code", "message", "details" } }` با کد HTTP درست.
- صفحه‌بندی: cursor-based (`?cursor=&limit=`، سقف ۵۰۰) + `Link` header.
- Idempotency: هدر `Idempotency-Key` روی عملیات write حساس (ساخت کاربر، پرداخت‌گونه، restore).
- Rate limit: per-token + per-IP، پاسخ `429` با `Retry-After`.
- OpenAPI: تولید از همان route registry (single source of truth) → `openapi.json` + `openapi.yaml`.

### ۳.۷ فرانت‌اند (استاک از قبل تصمیم‌گیری‌شده)
React 18 + TS + Vite + Tailwind + shadcn/ui. صفحات اصلی:
`Dashboard`، `Nodes`، `NodeDetail` (tabs: overview/health/logs/connections/install/config)、`Users`، `UserDetail`، `Bundles`، `Templates`، `Traffic`، `Certificates`، `Masks`، `Tunnels`، `Extensions`، `Notifications`، `Backups`، `Operators/Roles`، `Settings`، `AuditLog`، `Tools`، `Portal preview`.
- i18n انگلیسی + فارسی (RTL) + تقویم جلالی (قریب به همین حالا در پروژه تثبیت شده).
- نمای موبایل، جدول‌های مجازی برای لیست‌های بزرگ، Skeleton + optimistic update.

---

## ۴. فازبندی اجرایی

> قالب هر آیتم: `- [ ] **ID** — شرح — *وابستگی: X*`

### M0 — هسته، احراز هویت، RBAC، audit (پایهٔ همه‌چیز)
- [ ] **M0.1** اسکلت مونولیت + `cmd/ryxoui` + ساختار `internal/*` + Makefile + CI (build/test/lint/gofmt/vet).
- [ ] **M0.2** لایهٔ store: interface مشترک، پیاده‌سازی SQLite(modernc) و Postgres(pgx)، migration runner نسخه‌دار.
- [ ] **M0.3** اسکیمای اولیه: operators, roles, sessions, api_tokens, settings, audit_log, event_outbox.
- [ ] **M0.4** سرور HTTP: router، middleware زنجیره‌ای (request-id → log → recover → rate-limit → auth → scope).
- [ ] **M0.5** احراز هویت اپراتور: session cookie (HttpOnly, SameSite=Strict) + توکن Bearer برای API؛ hash رمز Argon2id.
- [ ] **M0.6** RBAC: role resolution + scope check در هر handler + نقش‌های پایه + نقش سفارشی (CRUD) + `GET /api/scopes`.
- [ ] **M0.7** TOTP: enroll/confirm/disable، کد بازیابی تک‌بارمصرف، بازیابی با session فعلی.
- [ ] **M0.8** audit_log: ثبت تغییرات با before/after، actor، ip، user-agent؛ صفحهٔ مرور + فیلتر facets.
- [ ] **M0.9** outbox رویدادها + صف تحویل پایه (webhook ساده، HMAC).
- [ ] **M0.10** UI: login، 2FA، shell (layout/nav)، صفحهٔ 404/403، تنظیمات اولیه، صفحهٔ اپراتورها/نقش‌ها.
- [ ] **M0.11** setup wizard: تعیین مدیر اولیه، basepath، listen، TLS mode.
- [ ] **M0.12** OpenAPI generator از route registry (JSON + YAML) + صفحهٔ مستندات داخل UI.

**DoD M0:** e2e: ثبت‌نام اپراتور → ورود → ساخت نقش → محافظت endpoint با scope → ثبت audit → خروج.

### M1 — نودها، SSH، موتور xray
- [x] **M1.1** مدل node + registry CRUD + محدودیت حجم/دورهٔ نود + multiplier. — ✅ `internal/nodes`، migration `0004_nodes.sql`
- [x] **M1.2** لایهٔ SSH: کلید اختصاصی per-node، known_hosts pinning، اجرای فرمان با timeout، SFTP برای کپی فایل. — ✅ `internal/ssh` + `internal/nodes/keys.go`
- [~] **M1.3** نصب از راه دور: job با وضعیت/stream لاگ، probe اولیه، انتخاب نسخه، systemd unit، uninstall. — 🟡 `internal/nodes/install.go` (job با step/progress/log، واحد systemd، probe اولیه، نصب موتور). مانده: uninstall و جریان لاگ زنده.
- [x] **M1.4** Health: heartbeat، version/arch/core، disk/mem/load samples، هشدار. — ✅ probe + collector پنج‌دقیقه‌ای (`internal/nodes/collector.go`). مانده: هشدارها.
- [x] **M1.5** موتور xray: adapter کامل `Engine` (Install/Render/Apply/UpsertUser/SuspendUser/RemoveUser/Usage/Health). — ✅ `internal/engines` (قرارداد) + `internal/engines/xray` + suite هم‌رفتاری `internal/engines/enginetest`. مانده: آمار ترافیک از stats API.
- [~] **M1.6** مدل پیکربندی: entrypoints / egresses / endpoints / templates / route. — 🟡 مدل و API آماده، صفحهٔ UI مانده.
- [ ] **M1.7** Preset catalogue: قالب‌های آمادهٔ inbound/outbound با clone-and-edit.
- [ ] **M1.8** ابزارهای کلید: keypair، reality، vless-encryption، ECH keypair (در M5 نهایی می‌شود).
- [ ] **M1.9** لاگ نود: tail با فیلتر سطح/زمان، ring buffer.
- [~] **M1.10** صفحهٔ نودها + تب detail (overview/health/logs/config) + جادوگر نصب. — 🟡 صفحهٔ نودها و تب‌های overview/health/config/SSH موجود. مانده: تب لاگ و جادوگر نصب.
- [ ] **M1.11** مسیر `/install/ryxoui.sh` + endpoint دانلود باینری + CA panel.

**DoD M1:** نصب خودکار xray روی یک VPS آزمایشی، ساخت entrypoint، ساخت کاربر آزمایشی، تأیید connection.

### M2 — کاربران، سهمیه، آمار، پورتال
- [ ] **M2.1** CRUD کاربر + وضعیت‌ها (active/disabled/expired/quota-exhausted) + group + remark + contact.
- [ ] **M2.2** bundle (پلن ترافیک): volume/days/speed/ip/device limit/auto-reset.
- [ ] **M2.3** quota engine: شمارش up/down per user، reset زمان‌بندی‌شده، هشدار سهمیه، auto-disable.
- [ ] **M2.4** templates: قالب کاربر/نود/بسته + انتساب/解除.
- [ ] **M2.5** accounting: جمع up/down از موتورها، batch writer، جدول‌های `user_usage`/`node_usage`.
- [ ] **M2.6** stats API: سری زمانی (ساعتی/روزانه) per node/user/tag + گزارش top-users + node-usage.
- [ ] **M2.7** subscription: توکن هش‌شده، فرمت‌های `URI` / `clash` / `sing-box` / `v2ray` / `qr`، basepath/sub_domain.
- [ ] **M2.8** portal: صفحهٔ HTML سبک + تم‌های CSS + RTL + لینک‌ها + QR + دیتای لینک (vless://, vmess://, trojan://, wireguard://, veil://).
- [ ] **M2.9** ابزارها: bulk (enable/disable/extend/reset)، generate، export (CSV/JSON)، summary داشبورد.
- [ ] **M2.10** user_devices: HWID limit، ثبت دستگاه، last_seen.
- [ ] **M2.11** UI: داشبورد (tiles قابل‌چینش)، لیست کاربران/بسته‌ها/آمار + نمودار.
- [ ] **M2.12** اپراتور چندگانه: محدودیت user_limit/max_duration/max_device_limit روی نقش.

**DoD M2:** ساخت ۱۰۰ کاربر، تخصیص سهمیه، مصرف ترافیک واقعی، دانلود اشتراک، مشاهدهٔ آمار.

### M3 — sing-box، WireGuard (+AmneziaWG)، OpenVPN
- [ ] **M3.1** adapter sing-box (inboundهای VLESS/VMess/Trojan/Hysteria2/TUIC/ShadowSocks + transportها: tcp/ws/grpc/http/h2/quic).
- [ ] **M3.2** adapter wireguard + amneziawg (kernel vs userspace انتخاب، per-user peer، stats).
- [ ] **M3.3** adapter openvpn (server/client-override، push routes).
- [ ] **M3.4** adapter ssh (sshd با محدودیت، per-user key، shell/pty policy).
- [ ] **M3.5** انتخاب موتور per-entrypoint و fallback chain.
- [ ] **M3.6** قالب‌های اشتراک چندپروتکلی + ترتیب و اولویت.
- [ ] **M3.7** تست هم‌تعاملی هر موتور (e2e: نصب → کاربر → connect → accounting).

**DoD M3:** ۴ موتور فعال، هرکدام با e2e pass.

### M4 — اعلان‌ها، بکاپ، OIDC، اتصال زنده
- [ ] **M4.1** outbox کامل: retry/backoff، dead-letter، replay، صفحهٔ deliveries + redeliver.
- [ ] **M4.2** webhook: HMAC، secret rotation، ping، فیلتر رویداد، per-subscription rate limit.
- [ ] **M4.3** ایمیل: SMTP config، template پیام (EN/FA)، recipientها با تأیید، تست ارسال، redeliver.
- [ ] **M4.4** تلگرام: pairing کد، chat ثبت‌شده، template پیام، تست.
- [ ] **M4.5** کاتالوگ رویداد کامل + UI کشف رویدادها.
- [ ] **M4.6** بکاپ: آرشیو zip رمزدار (age/AES)، شامل DB + تنظیمات + گواهی‌ها، schedule، retention، restore با validate قبل از جایگزینی.
- [ ] **M4.7** OIDC/SSO: provider CRUD + presets، link کردن identity به اپراتور، callback، نقشهٔ role از claim.
- [ ] **M4.8** اتصال زنده: لیست connectionهای فعال نود/کاربر، جزئیات، بستن اجباری.
- [ ] **M4.9** صفحهٔ اعلان‌ها + صفحهٔ deliveries.

**DoD M4:** یک رویداد از user.created تا Telegram/email/webhook با HMAC صحیح.

### M5 — UX، نقش سفارشی، dry-run، OpenAPI، Prometheus، ingress
- [ ] **M5.1** نقش سفارشی کامل + scope editor + inheritance + preview اثر.
- [ ] **M5.2** dry-run: نمایش diff کانفیگ قبل از Apply روی نود + rollback.
- [ ] **M5.3** OpenAPI: تولید خودکار، UI مستندات (Swagger-like) + publish.
- [ ] **M5.4** `/metrics` Prometheus + داشبورد Grafana نمونه.
- [ ] **M5.5** ingress/masks: fronting دامنه (SNI/Host header/ALPN/fingerprint) + bind به entrypoint.
- [ ] **M5.6** تم‌های UI (light/dark) + انتخاب + tiles داشبورد قابل‌چینش.
- [ ] **M5.7** i18n کامل + فارسی RTL + جلالی.
- [ ] **M5.8** ابزارها: link-name-preview، reality/ECH keypair، encryption helper.
- [ ] **M5.9** global search + کلیدهای میان‌بر + command palette.
- [ ] **M5.10** self-update panel (check/download/apply با تأیید) — ⚠️ **پیاده‌سازی شده در M0** (`internal/update` + `/api/panel/update/*` + صفحهٔ Updates + workflow انتشار)؛ در M5 فقط بهبود تجربه (به‌روزرسانی خودکار opt-in + تشخیص نسخه در داشبورد) باقی می‌ماند.

**DoD M5:** UI در RTL بدون شکست چیدمان؛ diff قابل‌بازبینی قبل از Apply؛ `/metrics` معتبر.

### M6 — MTProto، SSH-proxy، Mieru، TrustTunnel، Sudoku
- [ ] **M6.1** adapter mtproto (mtg): ساخت secret، per-user، انتقال به backend.
- [ ] **M6.2** proxy SSH: tunel معکوس به SSH-over-panel با policy.
- [ ] **M6.3** adapter mieru (mita/mbox): نصب سرویس، per-user key.
- [ ] **M6.4** adapter trusttunnel: endpoint Rust، credential/sub/deeplink.
- [ ] **M6.5** adapter sudoku-sing-box: type `sudoku`.
- [ ] **M6.6** پشتیبانی چند-موتوره در یک اشتراک + اولویت‌دهی.

**DoD M6:** هر ۵ موتور e2e pass و در پورتال دیده شوند.

### R — نقشهٔ راه (پس از M6)
- [ ] **R.1** Tunnels: reverse/relay، balance strategy، workers، stream window، health.
- [ ] **R.2** ACME کامل: DNS challenge (چند provider)، wildcard، panel cert + node cert، auto-renew + hook.
- [ ] **R.3** mTLS داخلی: چرخش کلید بین panel و node، enforcement.
- [ ] **R.4** rule_feeds: ویرایشگر قواعد، hash محتوا، rollback نسخه.
- [ ] **R.5** Extensions (افزونه‌ها): registry امضاشده، نصب، health، lifecycle، مجوزها.
- [ ] **R.6** ECH + مسیرهای ضد‌شناسایی.
- [ ] **R.7** WARP / outboundهای خاص.
- [ ] **R.8** IP limit پیشرفته + geo/ASN + قوانین سخت‌گیرانه.
- [ ] **R.9** اپراتور چند-tenant (reseller بدون لایهٔ مالی: فقط سهمیهٔ کاربر).
- [ ] **R.10** HA: Postgres + چند replica + انتخاب leader برای scheduler.
- [ ] **R.11** decoy web-server برای Veil (active probing).
- [ ] **R.12** SDK کلاینت Veil + GUI.

---

## ۵. کارهای مشترک (cross-cutting) — هر فاز
- [ ] **X.1** لایهٔ لاگ ساخت‌یافته با request-id + correlation.
- [ ] **X.2** Health/metrics داخلی پنل (`/api/panel/info`).
- [ ] **X.3** کتابخانهٔ خطای استاندارد + ترجمهٔ پیام‌ها (EN/FA).
- [ ] **X.4** تست: واحد + integration + e2e (testcontainers برای Postgres) + `-race` + fuzz روی parser کانفیگ.
- [ ] **X.5** CI: lint, vet, test, build چندپلتفرمی، SBOM، اسکن secret.
- [ ] **X.6** مستندات: راهنمای نصب، مرجع API (تولیدشده)، کاتالوگ رویداد، معماری.
- [ ] **X.7** migration رفت‌وبرگشت‌پذیر با تست upgrade از نسخهٔ قبل.
- [ ] **X.8** قابلیت **Auto Update**: پنل بتواند نسخهٔ جدید خود را از مخزن انتشار بگیرد، checksum را راستی‌آزمایی کند، باینری را جایگزین کند و سرویس را ری‌استارت کند — ✅ **انجام‌شده در M0** (`internal/update`، `update_state`، `/api/panel/update/{check,apply}`، صفحهٔ Updates، `.github/workflows/release.yml`). مانده: به‌روزرسانی خودکار opt-in، امضای دیجیتال artefact، به‌روزرسانی نودها از پنل.

---

## ۶. Definition of Done سراسری
1. هر endpoint جدید: scope صریح، پوشش پاسخ، مستند در OpenAPI، تست e2e.
2. هر جدول جدید: migration در هر دو موتور (SQLite/Postgres)، ایندکس‌های لازم، تست migration.
3. هر قابلیت UI: حالت‌های loading/empty/error، سازگاری موبایل، RTL، کیبورد.
4. هر قابلیت مخرب: confirm + audit + امکان rollback (در صورت امکان).
5. بدون نشتی credential: هیچ secret در لاگ، پاسخ API، یا audit (تست خودکار).

---

## ۷. ریسک‌ها
| ریسک | اثر | کاهش |
| --- | --- | --- |
| دامنهٔ بسیار بزرگ (۷۴۰ مسیر معادل) | scope creep | برش سخت به M0–M6؛ هر فاز ارزش مستقل دارد |
| چندموتوره بودن × N پروتکل | انفجار کد | قرارداد `Engine` واحد + conformance test مشترک |
| نداشتن محیط نود واقعی | عدم اعتبارسنجی | ۲ VPS آزمایشی + CI با container برای هر موتور |
| امنیت session/credential | افشای کامل | Argon2id، rotation، mTLS (R.3)، تست نشت |
| پیچیدگی i18n/RTL | UI ناسازگار | RTL از روز اول، آزمون چیدمان خودکار |

---

## ۸. گام بعدی (پیشنهاد)
1. تأیید این سند و انتخاب ترتیب شروع (پیشنهاد: **M0** با تمرکز روی store + auth + RBAC + audit).
2. نوشتن plan اجرایی M0 به سبک `docs/superpowers/plans/` (task-level، با فایل‌های هدف و معیار پذیرش).
3. شروع پیاده‌سازی M0.1–M0.4 در `D:\Projects\test\veil` (یا ریشهٔ جدید `ryxoui`).

---

## پیوست الف — شواهد خام
- `extracted/nexora-panel/openapi-summary.txt` — فهرست کامل ۲۱۹ مسیر
- `extracted/nexora-panel/event-summary.txt` — ۵۲ رویداد
- `extracted/nexora-panel/nexora-schema.txt` — مدل دادهٔ مرجع (فقط برای فهم دامنه)
- `extracted/nexora-panel/nexora-config.txt` — کلیدهای تنظیمات
- `extracted/nexora-panel/fe-endpoints.txt` — ۱۹۷ مسیر مصرف‌شدهٔ فرانت

> یادآوری: این پیوست‌ها **فقط‌خواندنی** و مرجعِ «دامنهٔ قابلیت» هستند؛ هیچ کد/متن/ظاهری از آن‌ها وارد RyxoUI نمی‌شود.
