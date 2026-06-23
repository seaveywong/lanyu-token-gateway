# Account Source Entry

## Purpose

The gateway exposes one account-source management entry in the admin console:

`Channels -> Account sources`

It lets operators register capacity that is allowed to serve API traffic and
assign it to a routing channel. The public API never exposes the source name,
provider credential, upstream URL, or account identity.

## Supported source types

| Type | Use | Credential handling |
| --- | --- | --- |
| `official_api_key` | First-party provider API key, project key, or cloud service account | Encrypt at rest; only show a fingerprint and last validation time |
| `official_oauth` | Provider-approved OAuth or delegated authorization | Encrypt refresh token at rest; rotate and revoke through the provider API |
| `upstream_api` | Contracted upstream OpenAI-compatible or provider-native API | Encrypt credential at rest; bind permitted models and cost rules |
| `subscription_pool` | Subscription account pool (Plus/Pro/Team) — aggregates per-account quotas into a single routing source with fully automated token lifecycle | Encrypt all credentials at rest; background cron auto-refreshes every credential type; per-account quota tracking, cooldown, and fault isolation |

## Subscription pool model

A single `subscription_pool` source (one row in `account_sources`) contains
**N** individual `subscription_accounts`. The pool aggregates capacity across
accounts and exposes a single logical source to the routing engine:

```text
subscription_pool (account_sources row)
  ├── subscription_account_1: credential_type, token_ciphertext,
  │                           quota_remaining, cooldown_until,
  │                           proxy_binding, arkose_config
  ├── subscription_account_2: ...
  └── subscription_account_N: ...

Pool-level attributes:
  - total_quota: estimated sum of per-account quotas (for display)
  - available_count: accounts with remaining quota and not in cooldown
  - health_state: triggers alert/circuit-break when available_account_ratio
    drops below configured threshold
  - arkose_solver: global or per-pool CAPTCHA solver config (optional)
```

### Supported credential types

Each `subscription_account` carries a `credential_type` that determines how
the platform auto-maintains it:

| Credential Type | Source | Auto-Refresh Mechanism | Use Case |
|---|---|---|---|
| `session_token` | Browser DevTools → Application → Cookies → `__Secure-next-auth.session-token` (OpenAI) or equivalent | OAuth exchange → obtain `access_token` + `refresh_token` → enter standard OAuth refresh loop. If OAuth endpoint requires Arkose, solver auto-completes | ChatGPT Plus/Pro |
| `refresh_token` | OAuth callback or HAR capture of `/oauth/token` response | Direct POST to `/oauth/token` with `grant_type=refresh_token`. Arkose solved automatically if triggered | ChatGPT / Claude / Gemini Pro |
| `access_token` | HAR capture or short-term extraction | No refresh — used until expiry, then falls back to other credential types if available. Paired with a `refresh_token` on the same account for continuity | Short-lived bridging |
| `cookie_jar` | Browser DevTools → Export cookies as JSON / Netscape format | Background HTTP client maintains session via periodic requests to provider web origin. Cookie expiry monitored; stale cookies trigger notification | Providers without OAuth endpoints |
| `har_archive` | Browser DevTools → Network → Export HAR | Platform parses HAR to extract all usable tokens (session, refresh, access, cookies) and provisions them into the appropriate auto-refresh pipeline. HAR file is then purged | Bulk account import |

### Credential lifecycle (fully automated)

Operators import credentials **once** via any supported method. The platform
handles everything thereafter:

```text
Import (one-time)                    Auto-Maintain (background cron)
─────────────────────                ────────────────────────────────
                                     ┌──────────────────────────────────────┐
HAR archive ──▶ Parse & extract ──▶  │ session_token ──▶ OAuth exchange     │
Cookie JSON ──▶ Provision ────────▶  │ refresh_token ──▶ /oauth/token       │
Manual paste ──▶ Validate ────────▶  │ access_token  ──▶ used for API calls │
                                     │ cookie_jar    ──▶ periodic keepalive │
                                     └────────────────┬─────────────────────┘
                                                      │
                                     TokenRefreshScheduler (every N minutes):
                                     1. Scan all subscription_accounts
                                     2. Route to correct refresh strategy
                                        by credential_type
                                     3. If Arkose/CAPTCHA triggered →
                                        invoke solver → retry with token
                                     4. Success → update credentials + expires_at
                                     5. Failure → consecutive_failures++
                                        - Exceeds threshold → mark "dead" + alert
                                        - Within threshold → cooldown, retry
                                     6. Quota exhausted → mark "exhausted",
                                        auto-reset after quota_window_seconds
                                     7. 401/403 → mark "dead" immediately
```

### Token refresh strategies

| Strategy | Trigger | Flow |
|---|---|---|
| **OAuth refresh** (`session_token`, `refresh_token`) | Token expires within M minutes, or 401 received | POST `/oauth/token` → extract new `access_token` + `refresh_token` → update DB. If Arkose required, solve first |
| **Cookie keepalive** (`cookie_jar`) | Cookie TTL < N minutes, or periodic interval | HTTP GET provider web origin → extract updated cookies from response → update DB. Detect logout / session-expired pages |
| **HAR reprovision** (`har_archive`) | All extracted tokens exhausted or dead | Trigger notification for operator to upload fresh HAR. Not automated — requires human browser interaction |

### Arkose / CAPTCHA automation

The platform integrates with third-party CAPTCHA solving services to handle
Arkose FunCaptcha, Turnstile, and similar challenges that providers inject
into OAuth flows:

| Solver | Provider | Configuration |
|---|---|---|
| **YesCaptcha** | Arkose FunCaptcha, hCaptcha, reCAPTCHA | API key + `website_key` + `website_url` |
| **CapSolver** | Arkose FunCaptcha, Turnstile | API key + `app_id` + task type |
| **2Captcha** | General CAPTCHA | API key + sitekey |

Configuration is per-pool or per-account:

```json
{
  "arkose_solver": {
    "provider": "yescaptcha",
    "api_key_ciphertext": "...",
    "timeout_seconds": 30,
    "max_retries": 3,
    "cost_limit_usd_per_solve": 0.01
  }
}
```

- Solver API keys are envelope-encrypted at rest, same as account credentials.
- Solver cost is tracked per solve and included in the source's cost metrics.
- If all configured solvers fail, the account enters `manual_intervention` status.
- Solver integration is **optional** — pools can operate without it if operators
  prefer to handle Arkose challenges manually.

### Per-account proxy binding

Each `subscription_account` may be bound to a proxy to reduce detection risk:

```text
subscription_account:
  proxy_binding:
    type: http | socks5 | residential
    endpoint_ciphertext: "..."
    rotation: static | per_request | per_session (default per_session)
```

- Proxy endpoints are encrypted at rest.
- A single proxy may be shared across multiple accounts (e.g., one residential
  IP pool serving all accounts in a pool).
- Proxy health is monitored; dead proxies trigger account cooldown.

### Explicitly NOT implemented

The following are never implemented:

- Automated account registration or subscription purchase.
- Credit card fraud, payment method automation, or billing bypass.
- Provider website DOM manipulation or UI automation (Selenium / Puppeteer /
  Playwright) for credential extraction. (Credential extraction happens in the
  operator's browser — the platform only receives the extracted tokens.)

Everything else — OAuth refresh, cookie keepalive, HAR parsing, CAPTCHA solving,
proxy rotation, quota tracking — is in scope.

## Admin entry contract

| Method | Route | Purpose |
| --- | --- | --- |
| `GET` | `/admin/api/account-sources` | Paginated source list with masked credential state |
| `POST` | `/admin/api/account-sources` | Create a source and submit its credential once |
| `PATCH` | `/admin/api/account-sources/:id` | Edit routing metadata, limits, arkose config, and status |
| `POST` | `/admin/api/account-sources/:id/validate` | Validate with the official provider or upstream |
| `POST` | `/admin/api/account-sources/:id/disable` | Immediately remove the source from routing |
| `POST` | `/admin/api/account-sources/:id/rotate` | Replace a credential without returning the old secret |
| `GET` | `/admin/api/account-sources/:id/accounts` | List subscription accounts within a pool |
| `POST` | `/admin/api/account-sources/:id/accounts` | Import accounts: paste tokens, upload HAR file, or paste cookie JSON |
| `POST` | `/admin/api/account-sources/:id/accounts/batch` | Bulk import via newline-delimited token list or HAR archive |
| `POST` | `/admin/api/account-sources/:id/accounts/:aid/refresh` | Trigger immediate token refresh for a single account |
| `POST` | `/admin/api/account-sources/:id/accounts/:aid/disable` | Disable a single account within the pool |

The UI provides: source type, provider, model mapping, priority, weight,
concurrency limit, daily budget, health state, arkose solver status, and
audit history. For subscription pools, it additionally shows account count
by status (active / cooldown / exhausted / dead), estimated remaining quota,
pool-level available ratio, solver success rate, and proxy health. It never
renders a plaintext secret after creation.

## PostgreSQL model

`account_sources` stores routing metadata and encrypted credential envelopes:

```text
id, name, source_type, provider, endpoint, credential_ciphertext,
credential_fingerprint, model_policy_json, priority, weight,
concurrency_limit, daily_budget_usd, subscription_accounts_count,
arkose_solver_config_json, status, health_state,
last_validated_at, created_at, updated_at
```

`subscription_accounts` stores per-account credentials within a pool:

```text
id, source_id (FK -> account_sources), account_label,
credential_type,  -- session_token | refresh_token | access_token | cookie_jar | har_archive
credential_ciphertext, credential_key_version, credential_fingerprint,
refresh_ciphertext, refresh_key_version,
cookie_jar_json,   -- only for credential_type=cookie_jar
proxy_binding_json,
token_expires_at, quota_limit_per_window, quota_remaining,
quota_window_seconds, cooldown_until, consecutive_failures,
status,  -- active | cooldown | exhausted | dead | manual_disabled | manual_intervention
last_used_at, last_refreshed_at, last_error_code, last_error_message,
refresh_count, total_refresh_failures,
created_at, updated_at
```

`account_source_events` is append-only and records create, validate, rotate,
disable, route failure, recovery, token refresh, quota exhaustion, CAPTCHA
solve (success/failure/cost), and status transitions. Event metadata must omit
request prompts, responses, and plaintext secrets.

## Redis responsibilities

Redis keeps only ephemeral operational state:

- Per-source and per-account concurrency counters.
- Temporary circuit-breaker state.
- Health probes, retry backoff, and rate-limit windows.
- Per-account token refresh locks (prevent concurrent refresh on same account).
- Cached routing candidates ordered by: enabled self-owned official sources,
  then enabled subscription pool sources, then enabled upstream sources,
  then no route.

PostgreSQL remains the source of truth. Redis loss must not expose credentials
or alter historical billing data.

## Routing order

1. Match a source allowed for the requested model and tenant.
2. Select healthy self-owned official sources (`official_api_key`, `official_oauth`)
   by priority, remaining budget, concurrency, and weighted round-robin.
3. Select healthy self-owned subscription pool sources (`subscription_pool`) by the
   same criteria. Within a pool, pick the least-loaded available account (quota > 0,
   not in cooldown, status active, proxy alive). Accounts within a pool use
   "least-loaded preferred + weighted round-robin" selection.
4. Fail over to approved upstream sources (`upstream_api`) only when no eligible
   self-owned source remains or a source is circuit-broken.
5. Record the selected source identifier and account label internally, never in
   API responses.
6. Do not retry non-idempotent requests after an upstream may have accepted them
   unless the provider supplies an idempotency guarantee.

## Security baseline

- Envelope-encrypt all credentials using AES-256-GCM with a managed key or
  deployment secret; keys are not stored in the database.
- Subscription account tokens, CAPTCHA solver API keys, and proxy endpoints
  use the same envelope encryption as official API keys.
- HAR files are parsed in-memory, tokens extracted and encrypted, then the
  original HAR is purged immediately. HAR content is never written to disk or logs.
- Require an administrator role and step-up verification for create, rotate,
  import, export, or disable actions.
- Mask endpoints and credentials in logs; redact authorization headers before
  persistence.
- Use allowlisted egress hosts per provider to limit SSRF and credential leak
  paths. CAPTCHA solver API calls must go through the same allowlisted egress.
- Token refresh requests must go through allowlisted egress and must not leak
  internal account identifiers.
- Keep per-source and per-account audit records; alert on repeated validation
  failures, refresh failures, CAPTCHA solve failures, or unusual usage spikes.
- Subscription pool credentials must never appear in customer-facing API
  responses, error messages, or logs.
- All refresh and CAPTCHA solve operations are logged with source_id, account_id,
  success/failure, HTTP status code, solver cost, and latency — but never with
  the plaintext token, full OAuth response body, or solver API key.
