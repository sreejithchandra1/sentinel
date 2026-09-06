# Sentinel HTTP API

All dashboard and automation traffic goes through `/api`. JSON in, JSON out. Default listen address is `0.0.0.0:8082`.

```bash
export BASE=http://localhost:8082
export TOKEN='paste-your-api-token'
```

Authenticated examples use a Bearer token. Cookie sessions from the dashboard (`sentinel_session`) work the same way if you pass `-b` / `credentials: include`.

---

## Conventions

| | |
|---|---|
| Content type | `application/json` |
| Time | RFC 3339 UTC unless noted |
| IDs | UUIDs |
| Errors | `{ "error": "message" }` with 4xx/5xx |
| Empty delete | `204 No Content`, no body |
| Create | `201 Created` plus the new object |
| Pagination | `limit` (default 20, max 100), `offset`, response `{ items, total, limit, offset }` |
| Stats period | `24h` (default), `7d`, `30d` |
| Interval | Check interval minimum **30 seconds** |
| Passwords | At least 8 characters, one letter and one number |

**Roles**

| Role | Who |
|------|-----|
| Any user | Logged-in viewer or admin |
| Admin | Platform admin or customer admin (`role=admin`) |
| Platform admin | Admin with empty `tenant_id` |
| Customer admin | Admin with a `tenant_id` |

Viewers can read monitors, performance, incidents, SLA, and their own tokens/profile. They cannot create or edit resources. Customer users only see their tenant. Platform admins see everything; pass `?customer=<tenant_id>` to scope lists.

**PUT vs pause.** `PUT /api/monitors/{id}` and `PUT /api/performance/targets/{id}` replace the stored fields from the body. To pause or resume without rewriting config, use the `/enabled` endpoints.

---

## Authentication

Two equivalent ways to call protected routes:

1. **API token** (preferred for scripts): `Authorization: Bearer <token>`
2. **Session cookie** after `POST /api/auth/login`: `Cookie: sentinel_session=...` (HttpOnly, 24h, SameSite=Lax)

Create tokens at **Settings → API Tokens** or `POST /api/settings/tokens`. The raw token is returned **once**.

Login is locked after **5** failed attempts for **15 minutes** (HTTP 429).

### Create a token (from a session)

```bash
# 1) Log in (sets sentinel_session)
curl -sS -c cookies.txt -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"your-password"}' \
  "$BASE/api/auth/login"

# 2) Create a token
curl -sS -b cookies.txt -H 'Content-Type: application/json' \
  -d '{"name":"ci"}' \
  "$BASE/api/settings/tokens"
```

```json
{
  "id": "…",
  "user_id": "…",
  "name": "ci",
  "prefix": "a1b2c3d4",
  "created_at": "2026-09-04T06:00:00Z",
  "token": "hex-secret-shown-only-here"
}
```

Use that `token` as `$TOKEN` below.

```bash
curl -sS -H "Authorization: Bearer $TOKEN" "$BASE/api/profile"
```

---

## Public (no auth)

### `GET /api/health`

```bash
curl -sS "$BASE/api/health"
# {"status":"ok"}
```

### `GET /api/public/branding`

Org name, tagline, and logo (data URL). Used on the login page.

```bash
curl -sS "$BASE/api/public/branding"
```

```json
{
  "company_name": "Acme",
  "tagline": "Infrastructure monitoring",
  "logo": "data:image/png;base64,…"
}
```

### `GET /api/public/status`

Public status page payload. Returns **404** if the status page is disabled.

```bash
curl -sS "$BASE/api/public/status"
```

```json
{
  "title": "System Status",
  "monitors": [
    {
      "id": "…",
      "name": "www",
      "type": "http",
      "status": "up",
      "last_checked_at": "2026-09-04T06:00:00Z"
    }
  ]
}
```

`status` is the last check (`up` / `down` / `degraded` / `unknown`) or `paused` when the monitor is disabled.

### `GET|POST /api/heartbeat/{token}`

Push check-in for a **heartbeat** monitor. No auth. **404** if the token is unknown or the monitor is paused.

```bash
curl -sS -X POST "$BASE/api/heartbeat/THE_TOKEN"
# or
curl -sS "$BASE/api/heartbeat/THE_TOKEN"
# {"ok":true}
```

---

## Auth session

### `POST /api/auth/login`

```bash
curl -sS -c cookies.txt -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"your-password"}' \
  "$BASE/api/auth/login"
```

Success without MFA:

```json
{ "ok": true }
```

MFA enabled:

```json
{
  "ok": true,
  "mfa_required": true,
  "challenge_id": "…",
  "email_hint": "a***@example.com",
  "message": "…"
}
```

Then verify (sets the session cookie):

### `POST /api/auth/mfa/verify`

```bash
curl -sS -c cookies.txt -H 'Content-Type: application/json' \
  -d '{"challenge_id":"…","code":"12345678"}' \
  "$BASE/api/auth/mfa/verify"
# {"ok":true}
```

### `POST /api/auth/mfa/resend`

```bash
curl -sS -H 'Content-Type: application/json' \
  -d '{"challenge_id":"…"}' \
  "$BASE/api/auth/mfa/resend"
```

### `POST /api/auth/logout`

```bash
curl -sS -b cookies.txt -X POST "$BASE/api/auth/logout"
# {"ok":true}
```

### `POST /api/auth/forgot-password`

Always returns the same success message (no email enumeration). Rate limit: 5 per hour per IP.

```bash
curl -sS -H 'Content-Type: application/json' \
  -d '{"email":"you@example.com"}' \
  "$BASE/api/auth/forgot-password"
# {"message":"Password reset link sent to your email."}
```

### `POST /api/auth/reset-password`

```bash
curl -sS -H 'Content-Type: application/json' \
  -d '{"token":"from-email-link","new_password":"Newpass12"}' \
  "$BASE/api/auth/reset-password"
# {"ok":true}
```

---

## Profile

**Auth:** any user.

### `GET /api/profile`

```bash
curl -sS -H "Authorization: Bearer $TOKEN" "$BASE/api/profile"
```

```json
{
  "id": "…",
  "username": "admin",
  "name": "Ada",
  "email": "ada@example.com",
  "mfa_enabled": false,
  "role": "admin",
  "tenant_id": ""
}
```

### `PUT /api/profile`

`current_password` is required. MFA requires an email on the profile.

```bash
curl -sS -X PUT -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{
    "current_password": "your-password",
    "name": "Ada Lovelace",
    "email": "ada@example.com",
    "username": "admin",
    "mfa_enabled": false
  }' \
  "$BASE/api/profile"
```

To change password, also send `"new_password": "Newpass12"`.

---

## Monitors

**List/get:** any user. **Create/update/delete/pause:** admin.

Types: `http`, `port`, `ssl`, `dns`, `heartbeat`.

Statuses: `up`, `down`, `degraded`, `unknown`.

### `GET /api/monitors`

Query: `tag`, `customer` (platform admin, tenant UUID).

```bash
curl -sS -H "Authorization: Bearer $TOKEN" "$BASE/api/monitors"
curl -sS -H "Authorization: Bearer $TOKEN" "$BASE/api/monitors?tag=production"
curl -sS -H "Authorization: Bearer $TOKEN" "$BASE/api/monitors?customer=TENANT_ID"
```

Each list item is a monitor plus `latest_response_time_ms` when a check exists.

### `POST /api/monitors`

Create is always **enabled**. Minimum `interval_seconds` is 30.

**HTTP**

```bash
curl -sS -X POST -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{
    "type": "http",
    "name": "Homepage",
    "url": "https://example.com",
    "method": "GET",
    "expected_status": 200,
    "interval_seconds": 60,
    "timeout_ms": 10000,
    "slow_threshold_ms": 3000,
    "follow_redirects": true,
    "keyword_must_exist": "",
    "keyword_must_not_exist": "",
    "notify_email": true,
    "notify_slack": true,
    "notify_webhooks": true,
    "alert_after_failures": 2,
    "tags": ["production"],
    "tenant_id": ""
  }' \
  "$BASE/api/monitors"
```

Optional HTTP fields: `expected_status_min` / `expected_status_max` (range), `request_body`, `request_headers` (raw header text), `http_username` / `http_password` (Basic Auth), `invert` (treat up as down), `alert_emails` (comma-separated override).

**Port**

```bash
curl -sS -X POST -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{
    "type": "port",
    "name": "SSH",
    "url": "example.com",
    "port": 22,
    "interval_seconds": 60,
    "timeout_ms": 5000
  }' \
  "$BASE/api/monitors"
```

**SSL**

```bash
curl -sS -X POST -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{
    "type": "ssl",
    "name": "example.com cert",
    "url": "example.com",
    "port": 443,
    "interval_seconds": 3600
  }' \
  "$BASE/api/monitors"
```

**DNS** — `url` is the domain; `config` is JSON:

```bash
curl -sS -X POST -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{
    "type": "dns",
    "name": "example.com DNS",
    "url": "example.com",
    "config": "{\"dns_records\":[\"A\",\"AAAA\",\"MX\"]}",
    "interval_seconds": 300
  }' \
  "$BASE/api/monitors"
```

**Heartbeat** — token is generated server-side. Ping `GET|POST /api/heartbeat/{heartbeat_token}`.

```bash
curl -sS -X POST -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{
    "type": "heartbeat",
    "name": "Nightly backup",
    "config": "{\"grace_seconds\":3600}",
    "interval_seconds": 60
  }' \
  "$BASE/api/monitors"
```

Customer admins cannot set `tenant_id`; it is forced to their customer. Platform admins assign with `"tenant_id": "<customer-uuid>"`.

### `GET /api/monitors/{id}`

```bash
curl -sS -H "Authorization: Bearer $TOKEN" "$BASE/api/monitors/MONITOR_ID"
```

Heartbeat monitors include `heartbeat_token`. HTTP passwords are redacted; `http_auth_set` is true when a password is stored.

### `PUT /api/monitors/{id}`

Same body as create. Send the full monitor (all fields you care about). Omitting fields can clear them.

```bash
curl -sS -X PUT -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{ ...same shape as POST... }' \
  "$BASE/api/monitors/MONITOR_ID"
```

### `PUT /api/monitors/{id}/enabled`

Pause or resume without changing other fields. History is kept.

```bash
curl -sS -X PUT -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"enabled":false}' \
  "$BASE/api/monitors/MONITOR_ID"

curl -sS -X PUT -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"enabled":true}' \
  "$BASE/api/monitors/MONITOR_ID"
```

### `DELETE /api/monitors/{id}`

Deletes the monitor and its incidents. `204`.

```bash
curl -sS -X DELETE -H "Authorization: Bearer $TOKEN" "$BASE/api/monitors/MONITOR_ID"
```

### `GET /api/monitors/{id}/results`

```bash
curl -sS -H "Authorization: Bearer $TOKEN" \
  "$BASE/api/monitors/MONITOR_ID/results?limit=20&offset=0"
```

`details` is a JSON string for SSL / DNS / port extra data.

### `GET /api/monitors/{id}/incidents`

Query: `limit`, `offset`, `from`, `to` (RFC 3339), `status` (`open` | `resolved`), `type` (`down`, `slow`, `ssl_expiry`, `dns_change`, `cert_change`).

```bash
curl -sS -H "Authorization: Bearer $TOKEN" \
  "$BASE/api/monitors/MONITOR_ID/incidents?status=open&limit=20"
```

### `GET /api/monitors/{id}/stats`

Query: `period=24h|7d|30d`.

```bash
curl -sS -H "Authorization: Bearer $TOKEN" \
  "$BASE/api/monitors/MONITOR_ID/stats?period=24h"
```

```json
{
  "monitor_id": "…",
  "points": [{ "timestamp": "…", "response_time_ms": 120, "status": "up" }],
  "uptime_pct": 99.9,
  "avg_response_ms": 118,
  "performance": {
    "avg_ms": 118, "min_ms": 90, "max_ms": 400,
    "p50_ms": 110, "p95_ms": 200, "p99_ms": 350,
    "slow_count": 2, "degraded_pct": 1.2
  }
}
```

### `POST /api/monitors/stats`

Compact uptime + sparkline for the list page. Max 200 IDs.

```bash
curl -sS -X POST -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"period":"30d","ids":["MONITOR_ID"]}' \
  "$BASE/api/monitors/stats"
```

```json
{
  "MONITOR_ID": { "uptime_pct": 99.95, "points": [110, 108, 250] }
}
```

Empty `ids` means all monitors the caller can access. Platform admins may send `"customer": "TENANT_ID"`.

---

## Performance targets

Latency probes, separate from uptime monitors. **List/get:** any user. **Write:** admin. Interval minimum 30 seconds.

### `GET /api/performance`

Fleet percentiles. Query: `period`, `customer`.

```bash
curl -sS -H "Authorization: Bearer $TOKEN" "$BASE/api/performance?period=24h"
```

### `GET /api/performance/targets`

```bash
curl -sS -H "Authorization: Bearer $TOKEN" "$BASE/api/performance/targets"
curl -sS -H "Authorization: Bearer $TOKEN" "$BASE/api/performance/targets?customer=TENANT_ID"
```

### `POST /api/performance/targets`

```bash
curl -sS -X POST -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{
    "name": "API latency",
    "url": "https://api.example.com/health",
    "method": "GET",
    "interval_seconds": 300,
    "timeout_ms": 10000,
    "slow_threshold_ms": 3000,
    "follow_redirects": true,
    "alert_after_slow": 2,
    "tenant_id": ""
  }' \
  "$BASE/api/performance/targets"
```

### `GET /api/performance/targets/{id}`

```bash
curl -sS -H "Authorization: Bearer $TOKEN" "$BASE/api/performance/targets/TARGET_ID"
```

### `PUT /api/performance/targets/{id}`

Full field update (same shape as create, plus `enabled`).

```bash
curl -sS -X PUT -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{ ... }' \
  "$BASE/api/performance/targets/TARGET_ID"
```

### `PUT /api/performance/targets/{id}/enabled`

```bash
curl -sS -X PUT -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"enabled":false}' \
  "$BASE/api/performance/targets/TARGET_ID"
```

### `DELETE /api/performance/targets/{id}`

```bash
curl -sS -X DELETE -H "Authorization: Bearer $TOKEN" "$BASE/api/performance/targets/TARGET_ID"
```

### `GET /api/performance/targets/{id}/results`

Query: `limit`, `offset`, `from`, `to`, `breaches` (`1` = SLA breaches only; `0` = all).

```bash
curl -sS -H "Authorization: Bearer $TOKEN" \
  "$BASE/api/performance/targets/TARGET_ID/results?breaches=1&limit=20"
```

### `GET /api/performance/targets/{id}/stats`

```bash
curl -sS -H "Authorization: Bearer $TOKEN" \
  "$BASE/api/performance/targets/TARGET_ID/stats?period=7d"
```

---

## Incidents

**Auth:** any user (tenant scoped).

Types: `down`, `slow`, `ssl_expiry`, `dns_change`, `cert_change`. Recovery emails are not listed as incidents unless you filter `type=recovery`.

### `GET /api/incidents`

Query: `limit`, `offset`, `open=1`, `status=open|resolved`, `type`, `monitor_id`, `from`, `to`.

```bash
curl -sS -H "Authorization: Bearer $TOKEN" "$BASE/api/incidents?open=1&limit=20"
```

```json
{
  "items": [
    {
      "id": "…",
      "monitor_id": "…",
      "monitor_name": "Homepage",
      "type": "down",
      "message": "connection refused",
      "started_at": "2026-09-04T05:00:00Z",
      "acknowledged_at": null,
      "acknowledged_by": ""
    }
  ],
  "total": 1,
  "limit": 20,
  "offset": 0
}
```

### `GET /api/incidents/{id}`

```bash
curl -sS -H "Authorization: Bearer $TOKEN" "$BASE/api/incidents/INCIDENT_ID"
```

### `POST /api/incidents/{id}/acknowledge`

Marks acknowledged; does not resolve. Idempotent.

```bash
curl -sS -X POST -H "Authorization: Bearer $TOKEN" \
  "$BASE/api/incidents/INCIDENT_ID/acknowledge"
```

---

## SLA report

**Auth:** any user.

Query: `month=YYYY-MM` (default current month UTC), `customer` (platform admin).

```bash
curl -sS -H "Authorization: Bearer $TOKEN" "$BASE/api/reports/sla?month=2026-09"
```

```json
{
  "period_start": "2026-09-01T00:00:00Z",
  "period_end": "2026-09-04T06:00:00Z",
  "monitor_count": 12,
  "incident_count": 3,
  "downtime_seconds": 420,
  "mttr_seconds": 140,
  "availability_pct": 99.95,
  "window_seconds": 280800,
  "monitors": []
}
```

---

## Customers

**Auth:** platform admin. `monitor_quota` is the max monitors for that tenant.

### `GET /api/settings/customers`

```bash
curl -sS -H "Authorization: Bearer $TOKEN" "$BASE/api/settings/customers"
```

### `POST /api/settings/customers`

Create does not set alert emails; use PUT after.

```bash
curl -sS -X POST -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"name":"Acme","monitor_quota":10}' \
  "$BASE/api/settings/customers"
```

### `PUT /api/settings/customers/{id}`

```bash
curl -sS -X PUT -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"name":"Acme","monitor_quota":15,"alert_emails":"ops@acme.com"}' \
  "$BASE/api/settings/customers/CUSTOMER_ID"
```

### `POST /api/settings/customers/{id}/test-email`

Sends a test using that customer’s recipient list (or the body override). Rate limited.

```bash
curl -sS -X POST -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"alert_emails":"ops@acme.com"}' \
  "$BASE/api/settings/customers/CUSTOMER_ID/test-email"
```

### `DELETE /api/settings/customers/{id}`

```bash
curl -sS -X DELETE -H "Authorization: Bearer $TOKEN" \
  "$BASE/api/settings/customers/CUSTOMER_ID"
```

---

## Team

**Auth:** admin. Customer admins only manage users in their tenant. Roles: `admin`, `viewer`.

### `GET /api/settings/team`

```bash
curl -sS -H "Authorization: Bearer $TOKEN" "$BASE/api/settings/team"
```

### `POST /api/settings/team`

```bash
curl -sS -X POST -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{
    "username": "sam",
    "email": "sam@example.com",
    "password": "Welcome12",
    "role": "viewer",
    "tenant_id": "CUSTOMER_ID"
  }' \
  "$BASE/api/settings/team"
```

Omit `tenant_id` for a platform user.

### `PUT /api/settings/team/{id}`

```bash
curl -sS -X PUT -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"username":"sam","email":"sam@example.com","role":"admin","tenant_id":"CUSTOMER_ID"}' \
  "$BASE/api/settings/team/USER_ID"
```

### `POST /api/settings/team/{id}/unlock`

Clears login lockout.

```bash
curl -sS -X POST -H "Authorization: Bearer $TOKEN" \
  "$BASE/api/settings/team/USER_ID/unlock"
```

### `POST /api/settings/team/{id}/reset-password`

```bash
curl -sS -X POST -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"password":"Newpass12"}' \
  "$BASE/api/settings/team/USER_ID/reset-password"
```

### `DELETE /api/settings/team/{id}`

```bash
curl -sS -X DELETE -H "Authorization: Bearer $TOKEN" "$BASE/api/settings/team/USER_ID"
```

---

## Notifications

### `GET /api/settings/notifications`

**Auth:** admin. Slack is tenant-scoped (customer vs platform). Email/webhooks summary is platform SMTP / global webhooks. `webhooks` is only included for platform admins.

```bash
curl -sS -H "Authorization: Bearer $TOKEN" "$BASE/api/settings/notifications"
```

```json
{
  "slack": { "enabled": true, "configured": true, "webhook_url": "https://hooks.slack.com/…", "events": ["all"] },
  "email": { "enabled": true, "configured": true },
  "webhooks": { "enabled": true, "configured": true, "count": 1 }
}
```

### Customer alert recipients (customer admin only)

Platform admins get **403**; they edit emails on the customer record instead.

```bash
curl -sS -H "Authorization: Bearer $TOKEN" "$BASE/api/settings/alert-recipients"
# {"alert_emails":"ops@acme.com"}

curl -sS -X PUT -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"alert_emails":"ops@acme.com,oncall@acme.com"}' \
  "$BASE/api/settings/alert-recipients"

curl -sS -X POST -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"alert_emails":"ops@acme.com"}' \
  "$BASE/api/settings/alert-recipients/test"
```

### Slack — `GET|PUT /api/settings/slack`, `POST /api/settings/slack/test`

**Auth:** admin. Customer admins write their tenant webhook; platform admins write the platform webhook.

`events`: `["all"]` or names matching alert types (`DOWN`, `RECOVERY`, `SLOW`, …). Empty list means all events.

```bash
curl -sS -H "Authorization: Bearer $TOKEN" "$BASE/api/settings/slack"

curl -sS -X PUT -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"webhook_url":"https://hooks.slack.com/services/…","enabled":true,"events":["all"]}' \
  "$BASE/api/settings/slack"

curl -sS -X POST -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{}' \
  "$BASE/api/settings/slack/test"
```

---

## Settings (platform admin unless noted)

### Organization — `GET|PUT /api/settings/general`, `POST /api/settings/general/reset`

`GET` is any authenticated user. `PUT` / `reset` are platform admin. Logo is a `data:` URL, max 512KB in the UI.

```bash
curl -sS -H "Authorization: Bearer $TOKEN" "$BASE/api/settings/general"

curl -sS -X PUT -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"company_name":"Acme","tagline":"Always on","logo":""}' \
  "$BASE/api/settings/general"

curl -sS -X POST -H "Authorization: Bearer $TOKEN" "$BASE/api/settings/general/reset"
```

### SMTP — `GET|PUT /api/settings/smtp`, `POST /api/settings/smtp/test`

Test is limited to 3 per minute. Password: omit or blank on PUT to keep the stored password.

```bash
curl -sS -H "Authorization: Bearer $TOKEN" "$BASE/api/settings/smtp"

curl -sS -X PUT -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{
    "host": "smtp.example.com",
    "port": 587,
    "username": "alerts@example.com",
    "password": "secret",
    "from": "alerts@example.com",
    "alert_emails": "ops@example.com",
    "tls": true,
    "enabled": true
  }' \
  "$BASE/api/settings/smtp"

curl -sS -X POST -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"to":"you@example.com"}' \
  "$BASE/api/settings/smtp/test"
# {"ok":true}
```

`alert_emails` is the **platform** CC list. Customer monitors use the customer’s list and still CC this list.

### Email log — `GET /api/settings/smtp/log`

Query: `limit`, `offset`, `status` (`sent` | `fail` | `skip` | `pending`), `kind` (`alert` | `test` | `password` | `mfa`), `from`, `to`.

```bash
curl -sS -H "Authorization: Bearer $TOKEN" \
  "$BASE/api/settings/smtp/log?status=fail&limit=20"
```

### Webhooks — `GET|PUT /api/settings/webhooks`

Global outbound hooks. PUT replaces the whole list. `events`: `["all"]` or specific alert types.

```bash
curl -sS -H "Authorization: Bearer $TOKEN" "$BASE/api/settings/webhooks"

curl -sS -X PUT -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '[{"url":"https://example.com/hook","enabled":true,"events":["all"]}]' \
  "$BASE/api/settings/webhooks"
```

### Maintenance — `GET|POST /api/settings/maintenance`, `DELETE /api/settings/maintenance/{id}`

Optional `monitor_id`: empty means all monitors. Times are RFC 3339; `ends_at` must be after `starts_at`.

```bash
curl -sS -H "Authorization: Bearer $TOKEN" "$BASE/api/settings/maintenance"

curl -sS -X POST -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{
    "name": "DB failover",
    "monitor_id": "",
    "starts_at": "2026-09-04T22:00:00Z",
    "ends_at": "2026-09-04T23:00:00Z"
  }' \
  "$BASE/api/settings/maintenance"

curl -sS -X DELETE -H "Authorization: Bearer $TOKEN" \
  "$BASE/api/settings/maintenance/WINDOW_ID"
```

### Server — `GET|PUT /api/settings/server`

`retention_days` minimum 30. Worker/retention changes need a process restart; `dashboard_url` applies immediately (password-reset and alert links).

```bash
curl -sS -H "Authorization: Bearer $TOKEN" "$BASE/api/settings/server"

curl -sS -X PUT -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"dashboard_url":"https://monitor.example.com","retention_days":90,"workers":10}' \
  "$BASE/api/settings/server"
```

### Status page — `GET|PUT /api/settings/status-page`

```bash
curl -sS -H "Authorization: Bearer $TOKEN" "$BASE/api/settings/status-page"

curl -sS -X PUT -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"enabled":true,"title":"Acme Status","monitor_ids":["MONITOR_ID"]}' \
  "$BASE/api/settings/status-page"
```

### Audit — `GET /api/settings/audit`, `GET /api/settings/audit/meta`

Query: `limit`, `offset`, `actor`, `action`, `resource`, `from`, `to`.

```bash
curl -sS -H "Authorization: Bearer $TOKEN" "$BASE/api/settings/audit/meta"
curl -sS -H "Authorization: Bearer $TOKEN" \
  "$BASE/api/settings/audit?action=pause&limit=20"
```

---

## API tokens

**Auth:** any user. Tokens belong to the caller only.

### `GET /api/settings/tokens`

```bash
curl -sS -H "Authorization: Bearer $TOKEN" "$BASE/api/settings/tokens"
```

The list never includes the secret, only `prefix`.

### `POST /api/settings/tokens`

```bash
curl -sS -X POST -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"name":"deploy"}' \
  "$BASE/api/settings/tokens"
```

Save `token` immediately.

### `DELETE /api/settings/tokens/{id}`

```bash
curl -sS -X DELETE -H "Authorization: Bearer $TOKEN" \
  "$BASE/api/settings/tokens/TOKEN_ID"
```

---

## Error examples

```json
{ "error": "unauthorized" }
```

| Status | Typical meaning |
|--------|-----------------|
| 400 | Invalid JSON or validation (`enabled required`, `name and url required`, …) |
| 401 | Missing/invalid session or Bearer token |
| 403 | Authenticated but wrong role (viewer write, customer hitting a platform-only route) |
| 404 | Unknown id, or a customer admin probing another tenant (returned as not found) |
| 429 | Login lockout, MFA/SMTP/forgot-password rate limits |
| 500 | `{ "error": "internal error" }` |

---

## Endpoint index

| Method | Path | Auth |
|--------|------|------|
| GET | `/api/health` | Public |
| GET | `/api/public/branding` | Public |
| GET | `/api/public/status` | Public |
| GET, POST | `/api/heartbeat/{token}` | Public |
| POST | `/api/auth/login` | Public |
| POST | `/api/auth/logout` | Public |
| POST | `/api/auth/mfa/verify` | Public |
| POST | `/api/auth/mfa/resend` | Public |
| POST | `/api/auth/forgot-password` | Public |
| POST | `/api/auth/reset-password` | Public |
| GET, PUT | `/api/profile` | Any user |
| GET | `/api/monitors` | Any user |
| POST | `/api/monitors` | Admin |
| GET | `/api/monitors/{id}` | Any user |
| PUT | `/api/monitors/{id}` | Admin |
| PUT | `/api/monitors/{id}/enabled` | Admin |
| DELETE | `/api/monitors/{id}` | Admin |
| GET | `/api/monitors/{id}/results` | Any user |
| GET | `/api/monitors/{id}/incidents` | Any user |
| GET | `/api/monitors/{id}/stats` | Any user |
| POST | `/api/monitors/stats` | Any user |
| GET | `/api/performance` | Any user |
| GET, POST | `/api/performance/targets` | GET any / POST admin |
| GET | `/api/performance/targets/{id}` | Any user |
| PUT | `/api/performance/targets/{id}` | Admin |
| PUT | `/api/performance/targets/{id}/enabled` | Admin |
| DELETE | `/api/performance/targets/{id}` | Admin |
| GET | `/api/performance/targets/{id}/results` | Any user |
| GET | `/api/performance/targets/{id}/stats` | Any user |
| GET | `/api/incidents` | Any user |
| GET | `/api/incidents/{id}` | Any user |
| POST | `/api/incidents/{id}/acknowledge` | Any user |
| GET | `/api/reports/sla` | Any user |
| GET | `/api/settings/general` | Any user |
| PUT | `/api/settings/general` | Platform admin |
| POST | `/api/settings/general/reset` | Platform admin |
| GET, PUT | `/api/settings/smtp` | Platform admin |
| POST | `/api/settings/smtp/test` | Platform admin |
| GET | `/api/settings/smtp/log` | Platform admin |
| GET | `/api/settings/notifications` | Admin |
| GET, PUT | `/api/settings/alert-recipients` | Customer admin |
| POST | `/api/settings/alert-recipients/test` | Customer admin |
| GET, PUT | `/api/settings/slack` | Admin |
| POST | `/api/settings/slack/test` | Admin |
| GET, POST | `/api/settings/team` | Admin |
| PUT | `/api/settings/team/{id}` | Admin |
| POST | `/api/settings/team/{id}/unlock` | Admin |
| POST | `/api/settings/team/{id}/reset-password` | Admin |
| DELETE | `/api/settings/team/{id}` | Admin |
| GET, POST | `/api/settings/customers` | Platform admin |
| PUT | `/api/settings/customers/{id}` | Platform admin |
| POST | `/api/settings/customers/{id}/test-email` | Platform admin |
| DELETE | `/api/settings/customers/{id}` | Platform admin |
| GET, PUT | `/api/settings/webhooks` | Platform admin |
| GET, POST | `/api/settings/maintenance` | Platform admin |
| DELETE | `/api/settings/maintenance/{id}` | Platform admin |
| GET, PUT | `/api/settings/server` | Platform admin |
| GET, PUT | `/api/settings/status-page` | Platform admin |
| GET | `/api/settings/audit` | Platform admin |
| GET | `/api/settings/audit/meta` | Platform admin |
| GET, POST | `/api/settings/tokens` | Any user |
| DELETE | `/api/settings/tokens/{id}` | Any user |
