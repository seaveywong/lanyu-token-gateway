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

Consumer web subscriptions, browser passwords, cookies, session tokens, and
automation around MFA or CAPTCHA are not valid source types. A Plus or Pro web
subscription must not be treated as an API credential unless its provider
offers an explicit API authorization mechanism.

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

The UI provides: source type, provider, model mapping, priority, weight,
concurrency limit, daily budget, health state, and audit history. It never
renders a plaintext secret after creation.

## PostgreSQL model

`account_sources` stores routing metadata and encrypted credential envelopes:

```text
id, name, source_type, provider, endpoint, credential_ciphertext,
credential_fingerprint, model_policy_json, priority, weight,
concurrency_limit, daily_budget_usd, status, health_state,
last_validated_at, created_at, updated_at
```

`account_source_events` is append-only and records create, validate, rotate,
disable, route failure, and recovery events. Event metadata must omit request
prompts, responses, and plaintext secrets.

## Redis responsibilities

Redis keeps only ephemeral operational state:

- Per-source concurrency counters and temporary circuit-breaker state.
- Health probes, retry backoff, and rate-limit windows.
- Cached routing candidates ordered by: enabled self-owned official sources,
  then enabled upstream sources, then no route.

PostgreSQL remains the source of truth. Redis loss must not expose credentials
or alter historical billing data.

## Routing order

1. Match a source allowed for the requested model and tenant.
2. Select healthy self-owned official sources by priority, remaining budget,
   concurrency, and weighted round-robin.
3. Fail over to approved upstream sources only when no eligible self-owned
   source remains or a source is circuit-broken.
4. Record the selected source identifier internally, never in API responses.
5. Do not retry non-idempotent requests after an upstream may have accepted
   them unless the provider supplies an idempotency guarantee.

## Security baseline

- Envelope-encrypt credentials using a managed key or deployment secret; keys
  are not stored in the database.
- Require an administrator role and step-up verification for create, rotate,
  export, or disable actions.
- Mask endpoints and credentials in logs; redact authorization headers before
  persistence.
- Use allowlisted egress hosts per provider to limit SSRF and credential leak
  paths.
- Keep per-source audit records and alert on repeated validation failures or
  unusual usage spikes.
