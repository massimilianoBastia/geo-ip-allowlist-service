# API reference

The detailed description of the HTTP API of geo-ip-allowlist-service. The README has the short version: [README.md](../README.md#api-specification).

## Which file is the authority

Each fact lives in one place. If two files disagree, the one higher in this list wins, and the lower one is a bug to fix.

| Order | File | Authority for |
|---|---|---|
| 1 | [api-design-notes.md](api-design-notes.md) | Every decision and its reason: reasons, `detail` texts, statuses, error precedence, logging policy, settings |
| 2 | [api/openapi.yaml](../api/openapi.yaml) | The machine-readable contract: paths, schemas, one example per status |
| 3 | [problems.md](problems.md) | The list of problem types and the `invalid_params` texts |
| 4 | this file | A readable explanation of the three above, plus examples run against the service |

This file adds no rule of its own. The examples marked "run" were produced by starting the service with `testdata/GeoIP2-Country-Test.mmdb`; the others are copied from `api/openapi.yaml`.

## Conventions

- Plain HTTP, no authentication. Access is limited by network policy to the gateway ([deployment.md](deployment.md)). The IP address the gateway sends is trusted.
- Requests and successful answers are JSON (`application/json`). Errors are `application/problem+json` ([RFC 9457](https://www.rfc-editor.org/rfc/rfc9457)).
- JSON member names are `snake_case`.
- The client has three states: allowed, not allowed, error. **Any 4xx or 5xx is "error".** A check that completes is always `200`, also when the IP is blocked.
- The `ip_address` value is never echoed in a response and never written to a log line. One exception: an invalid entry in `allowed_countries` is quoted back to the caller in the 422 `reason`, as the caller sent it (it is not logged).
- Unknown paths get the standard library `404 page not found` (`text/plain`), not a problem document.

## Endpoints

| Method and path | Purpose | Success |
|---|---|---|
| `POST /v1/ip-checks` | Is this IP in an allowed country? | `200` |
| `GET`, `HEAD /livez` | Process is alive | `200` always |
| `GET`, `HEAD /readyz` | A database is loaded | `200`, or `503` |
| `GET`, `HEAD /version` | Service version and database build date | `200` |
| `GET`, `HEAD /metrics` | Prometheus text format | `200` |

## `POST /v1/ip-checks`

### Request

Header `Content-Type: application/json` (a `charset` parameter is accepted). Body:

| Field | Type | Required | Rules |
|---|---|---|---|
| `ip_address` | string | yes | IPv4 or IPv6 text address, parsed with Go's `net/netip`. Rejected: an empty string, leading zeros (`010.0.0.1`), surrounding spaces, `999.1.1.1`. An IPv6 zone (`fe80::1%eth0`) is not an error: it is dropped before the lookup. An IPv4-mapped IPv6 address (`::ffff:81.2.69.160`) is looked up as IPv4. |
| `allowed_countries` | array of strings | yes | At least one entry. Each entry is two ASCII letters naming one of the 249 assigned ISO 3166-1 alpha-2 codes. Lower case is upper-cased (`gb` is `GB`). No trimming: `" GB"` is invalid. `UK`, `XK`, `USA`, `1` are invalid. A duplicate is allowed and gives a warning. |
| `session_id` | string | no | At most 128 characters (not bytes); a UUID is recommended. An empty string counts as absent. A longer value, or a value that is not a string, is ignored and gives the warning `invalid_session_id`. |
| `requested_at` | string | no | RFC 3339 timestamp, any offset, fractional seconds allowed. Only logged; never returned; it changes nothing. A bad value is ignored and gives the warning `invalid_requested_at`. |

Other rules:

- Unknown fields are ignored: never rejected, never a warning (only their count is logged at DEBUG, never their names). A misspelled required field counts as missing.
- If a key is repeated in the JSON object, the last one wins.
- Data after the JSON value (`{} {}`) is a `400`.

### Order of checks

Each check stops at the first failure.

1. Method is `POST`, else `405`.
2. `Content-Type` is `application/json`, else `415`.
3. Body is at most `MAX_REQUEST_BODY_BYTES`, else `413`.
4. Body is valid JSON, else `400`.
5. Field validation. **One `422` lists every problem**, `ip_address` first, then `allowed_countries` by ascending index.
6. A database is loaded, else `503`.
7. Lookup and decision.

The database check comes after validation, so an invalid request gets `422` even when no database is loaded; `503` applies only to a valid request.

### Response `200`

| Field | Type | Always present | Meaning |
|---|---|---|---|
| `allowed` | boolean | yes | `true` exactly when `reason` is `country_allowed` |
| `reason` | string | yes | One of the four values below. Switch on this, never on `detail`. |
| `detail` | string | yes | A sentence for people. It never contains the IP. Do not parse it. |
| `country` | object or `null` | yes | `{"iso_code":"GB","name":"United Kingdom"}`; `null` when no country is found. `name` is the English name from the database, informational only, and `""` if the database has none. |
| `session_id` | string | no | Echoed only when the request carried a valid one |
| `warnings` | array | no | Omitted when empty, never `[]` |

### The four reasons

| `reason` | `allowed` | `country` | Meaning |
|---|---|---|---|
| `country_allowed` | `true` | object | The IP is in a country that is in `allowed_countries` |
| `country_not_allowed` | `false` | object | The IP is in a country that is not in the list |
| `country_unknown` | `false` | `null` | No country can be found: private, loopback, link-local, unspecified or other reserved address; a public IP missing from the database; a record without a country. There is **no fallback** to `registered_country`: failing open would let anyone bypass the restriction. |
| `country_unsupported` | `false` | object | The IP resolved to a code that is not an ISO 3166-1 alpha-2 code (today only `XK`, Kosovo). No allowed list can contain it, so it is always blocked. |

Examples (run unless noted):

`country_allowed`. Request:

```json
{
  "ip_address": "81.2.69.160",
  "allowed_countries": [
    "GB",
    "DE"
  ]
}
```

Response `200`:

```json
{
  "allowed": true,
  "reason": "country_allowed",
  "detail": "The IP address is located in GB (United Kingdom), which is in allowed_countries.",
  "country": {
    "iso_code": "GB",
    "name": "United Kingdom"
  }
}
```

`country_not_allowed` (same IP, `"allowed_countries":["DE"]`):

```json
{
  "allowed": false,
  "reason": "country_not_allowed",
  "detail": "The IP address is located in GB (United Kingdom), which is not in allowed_countries, so it is blocked.",
  "country": {
    "iso_code": "GB",
    "name": "United Kingdom"
  }
}
```

`country_unknown`, private address (`10.0.0.1`):

```json
{
  "allowed": false,
  "reason": "country_unknown",
  "detail": "The IP address is in a private range and has no associated country, so it is blocked.",
  "country": null
}
```

`country_unknown`, a public IP with no country (`8.8.8.8` is not in the test database; `214.1.1.1` has a record without a country; `5.83.124.1` has only a `registered_country`, which is not used):

```json
{
  "allowed": false,
  "reason": "country_unknown",
  "detail": "No country is recorded for this IP address in the geolocation database, so it is blocked.",
  "country": null
}
```

`country_unknown`, IPv6 with a zone (`fe80::1%eth0` becomes `fe80::1`, link-local):

```json
{
  "allowed": false,
  "reason": "country_unknown",
  "detail": "The IP address is a loopback or link-local address and has no associated country, so it is blocked.",
  "country": null
}
```

The other `detail` texts for `country_unknown` are "The IP address is the unspecified address and has no associated country, so it is blocked." and "The IP address is in a reserved range and has no associated country, so it is blocked." (reserved covers multicast, carrier-grade NAT, documentation and other special ranges).

`country_unsupported` (copied from the contract; the test database has no `XK` record):

```json
{
  "allowed": false,
  "reason": "country_unsupported",
  "detail": "The IP address is located in a country whose code is not an ISO 3166-1 alpha-2 code, so it cannot be in any allowed list and is blocked.",
  "country": {
    "iso_code": "XK",
    "name": "Kosovo"
  }
}
```

When the database has a country code but no English name, `country.name` is `""` and the parentheses are left out of `detail` ("located in GB, which is in allowed_countries.").

### Warnings

A warning means "the request was answered, but part of it was not used". `warnings` is an array of `{code, detail, pointer}`; `pointer` is a JSON Pointer ([RFC 6901](https://www.rfc-editor.org/rfc/rfc6901)) to the part of the request. The list of codes is open: **clients ignore codes they do not know.**

| Code | Pointer | When | `detail` |
|---|---|---|---|
| `duplicate_country` | `/allowed_countries/<index>` of each repeated entry after the first | The same country appears twice, also after upper-casing (`["DE","de"]`) | `'IT' appears more than once in allowed_countries.` |
| `invalid_session_id` | `/session_id` | Longer than 128 characters, or not a string | `session_id must be a string of at most 128 characters; the value was ignored.` |
| `invalid_requested_at` | `/requested_at` | Not a string, or not RFC 3339 | `requested_at must be a valid RFC 3339 timestamp; the value was ignored.` |

Order: duplicates by index, then `/session_id`, then `/requested_at`. A duplicate never changes the decision.

Run, `"allowed_countries":["gb","GB"]` with `"session_id":"abc"`:

```json
{
  "allowed": true,
  "reason": "country_allowed",
  "detail": "The IP address is located in GB (United Kingdom), which is in allowed_countries.",
  "country": {
    "iso_code": "GB",
    "name": "United Kingdom"
  },
  "session_id": "abc",
  "warnings": [
    {
      "code": "duplicate_country",
      "detail": "'GB' appears more than once in allowed_countries.",
      "pointer": "/allowed_countries/1"
    }
  ]
}
```

Run, `"session_id":5` and `"requested_at":"yesterday"`:

```json
{
  "allowed": true,
  "reason": "country_allowed",
  "detail": "The IP address is located in GB (United Kingdom), which is in allowed_countries.",
  "country": {
    "iso_code": "GB",
    "name": "United Kingdom"
  },
  "warnings": [
    {
      "code": "invalid_session_id",
      "detail": "session_id must be a string of at most 128 characters; the value was ignored.",
      "pointer": "/session_id"
    },
    {
      "code": "invalid_requested_at",
      "detail": "requested_at must be a valid RFC 3339 timestamp; the value was ignored.",
      "pointer": "/requested_at"
    }
  ]
}
```

## Operational endpoints

`GET` and `HEAD` only. Any other method gets `405` with `Allow: GET, HEAD`. `HEAD` returns the same status and headers with no body.

| Endpoint | Answer |
|---|---|
| `/livez` | `200` `{"status":"ok"}` whenever the process runs, loaded database or not. Use it as the liveness probe. |
| `/readyz` | `200` `{"status":"ready","database_type":"GeoIP2-Country","database_built_at":"2026-10-22T18:52:57Z"}` when a database is loaded (run, test database; the real file reports `GeoLite2-Country`). Otherwise `503` `database-unavailable`. Use it as the readiness probe. |
| `/version` | `200` `{"version":"dev","database_built_at":"2026-10-22T18:52:57Z"}`. `version` is set at build time (`-X main.version=...`, default `dev`). `database_built_at` is left out while no database is loaded (run: `{"version":"dev"}`). |
| `/metrics` | `200`, `text/plain; version=0.0.4; charset=utf-8`. See below. |

`/metrics` has three series, all written from the first scrape with value `0` (except the timestamp):

```
ip_checks_total{reason="country_allowed"} 2
ip_checks_total{reason="country_not_allowed"} 1
ip_checks_total{reason="country_unknown"} 1
ip_checks_total{reason="country_unsupported"} 0
http_problems_total{type="malformed-request"} 1
...
database_build_timestamp_seconds 1792695177
```

`http_problems_total` has one line per problem type (including the `405` on operational endpoints). `database_build_timestamp_seconds` is left out while no database is loaded.

## Problem types

Every 4xx and 5xx from the check and the operational endpoints is `application/problem+json` with `type`, `title`, `status` and `detail`. `type` is the setting `PROBLEM_BASE_URL` (default `https://geo-service.example/problems/`, always ending in `/`) plus the name. Only `invalid-request` has the extra member `invalid_params`. The table of names, titles and details is in [problems.md](problems.md); here is one body for each (run, except 500 which cannot be triggered on purpose and is copied from the contract).

**400 `malformed-request`** (body `{bad`; also an empty body, plain text, or data after the JSON value):

```json
{
  "type": "https://geo-service.example/problems/malformed-request",
  "title": "Malformed request",
  "status": 400,
  "detail": "The request body is not valid JSON."
}
```

**405 `method-not-allowed`** on the check, with `Allow: POST` (`GET /v1/ip-checks`; `HEAD` too):

```json
{
  "type": "https://geo-service.example/problems/method-not-allowed",
  "title": "Method not allowed",
  "status": 405,
  "detail": "This resource only supports POST."
}
```

On `/livez`, `/readyz`, `/version`, `/metrics` the header is `Allow: GET, HEAD` and the detail is:

```json
{
  "type": "https://geo-service.example/problems/method-not-allowed",
  "title": "Method not allowed",
  "status": 405,
  "detail": "This resource only supports GET and HEAD."
}
```

**413 `request-too-large`** (a body of 16385 bytes with the default limit; the number in the text is the configured limit):

```json
{
  "type": "https://geo-service.example/problems/request-too-large",
  "title": "Request too large",
  "status": 413,
  "detail": "The request body exceeds the maximum allowed size of 16384 bytes."
}
```

**415 `unsupported-media-type`** (no `Content-Type`, or not `application/json`):

```json
{
  "type": "https://geo-service.example/problems/unsupported-media-type",
  "title": "Unsupported media type",
  "status": 415,
  "detail": "The request Content-Type must be application/json."
}
```

**422 `invalid-request`**, with `invalid_params`, a list of `{pointer, reason}` (JSON Pointer, no codes). Request `{"ip_address":"nope","allowed_countries":["USA"]}`:

```json
{
  "type": "https://geo-service.example/problems/invalid-request",
  "title": "Invalid request",
  "status": 422,
  "detail": "The request body is missing required fields or has invalid values.",
  "invalid_params": [
    {
      "pointer": "/ip_address",
      "reason": "ip_address must be a valid IPv4 or IPv6 address."
    },
    {
      "pointer": "/allowed_countries/0",
      "reason": "'USA' is not an ISO 3166-1 alpha-2 country code."
    }
  ]
}
```

A country entry in a `reason` is quoted as the caller sent it, even if the caller put an address there; the `ip_address` value is never quoted. Body `[]` (valid JSON, not an object) has the single pointer `""`:

```json
{
  "type": "https://geo-service.example/problems/invalid-request",
  "title": "Invalid request",
  "status": 422,
  "detail": "The request body is missing required fields or has invalid values.",
  "invalid_params": [
    {
      "pointer": "",
      "reason": "The request body must be a JSON object."
    }
  ]
}
```

All `invalid_params` texts are listed in [problems.md](problems.md#invalid-request).

**500 `internal-error`** (a lookup error or a panic; the body never holds internals; details go to the log):

```json
{
  "type": "https://geo-service.example/problems/internal-error",
  "title": "Internal error",
  "status": 500,
  "detail": "An unexpected error occurred."
}
```

**503 `database-unavailable`** (a valid check, or `/readyz`, while no database is loaded, for example at start while the first download is pending):

```json
{
  "type": "https://geo-service.example/problems/database-unavailable",
  "title": "Service unavailable",
  "status": 503,
  "detail": "The geolocation database is not loaded yet; retry later."
}
```

## Headers

| Header | Where | Value |
|---|---|---|
| `Content-Type` (request) | check | must be `application/json`, parameters such as `charset=utf-8` allowed |
| `Content-Type` (response) | `200` JSON | `application/json` |
| | problems | `application/problem+json` |
| | `/metrics` | `text/plain; version=0.0.4; charset=utf-8` |
| `Allow` | every `405` | `POST` on the check, `GET, HEAD` on operational endpoints |
| `Content-Length` | every answer except `HEAD` | set |

The service reads no other request header: no token, no `X-Forwarded-For`. The IP to check is always the one in the body.

## Limits

| Limit | Value | Setting |
|---|---|---|
| Request body | 16384 bytes. Exactly 16384 is answered normally; 16385 gets `413`. A declared `Content-Length` above the limit is refused before the body is read. | `MAX_REQUEST_BODY_BYTES` |
| `allowed_countries` length | no separate limit; the body limit bounds it. A valid list has at most 249 distinct entries; the rest are duplicates (warning) or invalid (`422`). | |
| `session_id` | 128 characters | fixed |
| Read header / read / write / idle timeout | 5 s / 10 s / 10 s / 60 s | fixed |
| Shutdown | on SIGTERM or SIGINT, requests in flight finish, up to this time | `SHUTDOWN_TIMEOUT` (default 10 s) |

An invalid `MAX_REQUEST_BODY_BYTES` (not a positive integer) stops the service at start with an error naming the variable. All settings are in [deployment.md](deployment.md).

## Logging and warnings

Logs are JSON lines on stdout, written with `log/slog`. **The IP address is never logged**, and no secret is. As a safety net, if an address were ever logged by mistake the guard type prints only a /24 prefix for IPv4 or /48 for IPv6.

One summary line `request handled` per check, with `method`, `path`, `status`, `duration_ms`, `allowed_countries_count`, and, when known, `reason`, `allowed`, `country_iso`, `requested_at` and `session_id` (a valid `session_id` is on every line of its request). Levels:

| Case | Level |
|---|---|
| `country_allowed`, `country_not_allowed`, any 4xx | INFO |
| `country_unknown` (with `address_category`: `private`, `loopback`, `link_local`, `unspecified`, `reserved` or `public_unmapped`), `country_unsupported` | WARN |
| Any 5xx, including `503` while no database is loaded | ERROR |
| Request lines for `/livez`, `/readyz`, `/version`, `/metrics`; the number of unknown fields | DEBUG |

An ignored optional field adds a WARN line `optional field ignored` with `field` and, instead of the value, its length (`session_id_length`) or its JSON type (`session_id_type`, for example `number`, `null`). A panic is an ERROR with the Go type of the panic value and the stack, not the value itself. Database events (loaded, rejected, waiting for the first file, sanity lookups off) are logged by the reloader; see [deployment.md](deployment.md). Set the level with `LOG_LEVEL`.

## Known limits of the answer

VPNs and proxies are not detected. 6to4, Teredo and NAT64 addresses are not unwrapped. `XK` can never be allowed. Full list: [api-design-notes.md](api-design-notes.md), section E.
