# Account Source Entry

## Purpose

The gateway exposes one account-source management entry in the future admin
console:

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
| `subscription_pool` | Subscription account pool (Plus/Pro/Team) aggregated by per-account quota | Encrypt token at rest; background cron auto-refreshes via standard OAuth; per-account quota tracking and cooldown; one-time import of session_token or refresh_token, no ongoing manual re-capture |

## Subscription pool model

A single `subscription_pool` source (one row in `account_sources`) contains
**N** individual `subscription_accounts`. The pool aggregates capacity across
accounts and exposes a single logical source to the routing engine:

```text
subscription_pool (account_sources row)
  ├── subscription_account_1: token_ciphertext, quota_remaining,
  │                           cooldown_until, token_expires_at
  ├── subscription_account_2: ...
  └── subscription_account_N: ...

Pool-level attributes:
  - total_quota: estimated sum of per-account quotas (for display)
  - available_count: accounts with remaining quota and not in cooldown
  - health_state: triggers alert/circuit-break when available_account_ratio
    drops below configured threshold
```

### Credential lifecycle (automated)

Operators import a credential **once** (session_token or refresh_token, obtained
via browser DevTools or approved extraction tool). The platform then maintains
the token lifecycle automatically through standard OAuth flows:

```text
Import (one-time)          Auto-Maintain (background cron)
─────────────────────      ────────────────────────────────
                           ┌──────────────────────────────┐
session_token ──▶ OAuth ──▶ access_token  (used for calls)│
                  exchange   refresh_token (persisted)     │
                           └──────────────┬───────────────┘
                                          │
                           Every N minutes, TokenRefreshScheduler:
                           1. Scan all subscription_accounts
                           2. If access_token expires within M minutes →
                              POST /oauth/token with refresh_token
                           3. Success → update credential + expires_at
                           4. Failure → consecutive_failures++
                              - Exceeds threshold → mark "dead" + alert
                              - Within threshold → mark "cooldown", retry
                           5. Quota exhausted → mark "exhausted",
                              reset after quota_window_seconds
                           6. Consecutive 401/403 → mark "dead" immediately
```

### Token refresh design (inspired by New API / Sub2API)

| Dimension | Design |
|---|---|
| **Scheduler** | Background goroutine / cron within `async-worker`, configurable interval (default 5 min) |
| **Advance window** | Refresh when token expires within M minutes (default 10 min) |
| **Concurrency** | Per-account mutex; a refreshing account is marked `schedulable=false` and skipped by the routing engine |
| **Arkose / CAPTCHA** | Not automated. If the OAuth endpoint requires Arkose, the account enters `manual_intervention` status and operators are notified. The platform does NOT integrate with CAPTCHA-solving services or browser automation |
| **Failure escalation** | 1–2 failures → cooldown 5 min. 3–5 failures → cooldown 30 min. 6+ → dead, manual review required |
| **Observability** | Metric `token_refresh_success_rate` per source; log every refresh attempt (success/failure/code) without plaintext secrets |

### Explicitly NOT implemented

The following are never implemented, even as hidden configuration:

- Username + password-based web login automation
- Browser automation (Selenium / Puppeteer / Playwright) for token extraction
- CAPTCHA recognition or MFA bypass
- Cookie jar persistence or automated cookie renewal
- Reverse-engineering of ChatGPT / Claude / Gemini web WebSocket protocols
- Automated account registration or subscription purchase

The platform only performs **standard OAuth 2.0 token refresh** against the
provider's official OAuth endpoint (e.g. `auth.openai.com/oauth/token`,
`auth.anthropic.com/oauth/token`). Any provider whose official OAuth does not
support `refresh_token` grant or whose web subscription terms explicitly
prohibit API access cannot be included in a subscription pool.

## Admin entry contract

The initial admin routes are reserved as follows:

| Method | Route | Purpose |
| --- | --- | --- |
| `GET` | `/admin/api/account-sources` | Paginated source list with masked credential state |
| `POST` | `/admin/api/account-sources` | Create a source and submit its credential once |
| `PATCH` | `/admin/api/account-sources/:id` | Edit routing metadata, limits, and status |
| `POST` | `/admin/api/account-sources/:id/validate` | Validate with the official provider or upstream |
| `POST` | `/admin/api/account-sources/:id/disable` | Immediately remove the source from routing |
| `POST` | `/admin/api/account-sources/:id/rotate` | Replace a credential or OAuth grant without returning the old secret |
| `GET` | `/admin/api/account-sources/:id/accounts` | List subscription accounts within a pool (subscription_pool only) |
| `POST` | `/admin/api/account-sources/:id/accounts` | Import one or more subscription accounts into a pool |
| `POST` | `/admin/api/account-sources/:id/accounts/:aid/refresh` | Trigger immediate token refresh for a single account |
| `POST` | `/admin/api/account-sources/:id/accounts/:aid/disable` | Disable a single account within the pool |

The UI provides: source type, provider, model mapping, priority, weight,
concurrency limit, daily budget, health state, and audit history. For
subscription pools, it additionally shows account count, available count,
cooldown count, and estimated remaining quota. It never renders a plaintext
secret after creation.

## PostgreSQL model

`account_sources` stores routing metadata and encrypted credential envelopes:

```text
id, name, source_type, provider, endpoint, credential_ciphertext,
credential_fingerprint, model_policy_json, priority, weight,
concurrency_limit, daily_budget_usd, subscription_accounts_count,
status, health_state, last_validated_at, created_at, updated_at
```

`subscription_accounts` stores per-account credentials within a pool:

```text
id, source_id (FK -> account_sources), account_label,
credential_type (session_token | refresh_token | access_token),
credential_ciphertext, credential_key_version, credential_fingerprint,
refresh_ciphertext, refresh_key_version,
token_expires_at, quota_limit_per_window, quota_remaining,
quota_window_seconds, cooldown_until, consecutive_failures,
status (active | cooldown | exhausted | dead | manual_disabled | manual_intervention),
last_used_at, last_refreshed_at, last_error_code, last_error_message,
created_at, updated_at
```

`account_source_events` is append-only and records create, validate, rotate,
disable, route failure, and recovery events. For subscription pools, it also
records token refresh successes, refresh failures, quota exhaustion, and
account status transitions. Event metadata must omit request prompts, responses,
and plaintext secrets.

## Redis responsibilities

Redis keeps only ephemeral operational state:

- Per-source concurrency counters and temporary circuit-breaker state.
- Per-account concurrency counters (subscription pool accounts).
- Health probes, retry backoff, and rate-limit windows.
- Cached routing candidates ordered by: enabled self-owned official sources,
  then enabled subscription pool sources, then enabled upstream sources,
  then no route.

PostgreSQL remains the source of truth. Redis loss must not expose credentials
or alter historical billing data.

## Routing order

1. Match a source allowed for the requested model and tenant.
2. Select healthy self-owned official sources (`official_api_key`, `official_oauth`) by priority, remaining budget, concurrency, and weighted round-robin.
3. Select healthy self-owned subscription pool sources (`subscription_pool`) by the same criteria. Within a pool, pick the least-loaded available account (remaining quota > 0, not in cooldown, active). Accounts within a pool use weighted round-robin with least-loaded preference.
4. Fail over to approved upstream sources (`upstream_api`) only when no eligible self-owned source remains or a source is circuit-broken.
5. Record the selected source identifier and account label internally, never in API responses.
6. Do not retry non-idempotent requests after an upstream may have accepted them unless the provider supplies an idempotency guarantee.

## Security baseline

- Envelope-encrypt credentials using a managed key or deployment secret; keys
  are not stored in the database.
- Subscription account tokens use the same envelope encryption as official
  API keys; no separate weak key material.
- Require an administrator role and step-up verification for create, rotate,
  export, or disable actions.
- Mask endpoints and credentials in logs; redact authorization headers before
  persistence.
- Use allowlisted egress hosts per provider to limit SSRF and credential leak
  paths.
- Token refresh requests must go through the same allowlisted egress and must
  not leak internal account identifiers.
- Keep per-source and per-account audit records; alert on repeated validation
  failures, refresh failures, or unusual usage spikes.
- Subscription pool credentials must never appear in customer-facing API
  responses, error messages, or logs.
- All refresh operations are logged with source_id, account_id, success/failure,
  HTTP status code, and latency — but never with the plaintext token or full
  OAuth response body.
