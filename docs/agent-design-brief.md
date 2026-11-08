# Agent design brief

The rules of the service itself, short and testable, for the next agents. The reasons and sources are in `docs/api-design-notes.md`; this brief holds only the rules. If this brief and the notes disagree, the notes win. How to work in this repository is in `CLAUDE.md`. Change order: notes, then `api/openapi.yaml`, then this brief.

Each rule cites the notes item it comes from, for example "R3 (A4)".

## Contract

`POST /v1/ip-checks` with a JSON body. Contract file: `api/openapi.yaml`.
Operational endpoints: `GET`/`HEAD` on `/livez`, `/readyz`, `/version`, and optionally `/metrics`.

## Rules

**Endpoint and response**
- R1 (A1) The check is `POST /v1/ip-checks`; any other method on it gets 405 with `Allow: POST`.
- R2 (A3) A completed check always returns HTTP 200, also when the result is "not allowed". The body has `allowed`, `reason`, `detail`, `country`, and only when present `session_id` and `warnings`.
- R3 (A3) `allowed` is true exactly when `reason` is `country_allowed`.
- R4 (A4) `reason` is one of exactly `country_allowed`, `country_not_allowed`, `country_unknown`, `country_unsupported`. Clients never parse `detail`.
- R5 (A4) Every case where no country is found (private, loopback, link-local, unspecified, multicast and other reserved ranges, a public IP missing from the data, a record without a country) returns `country_unknown`, `allowed` false, `country` null.
- R6 (A4) Never use `registered_country` or `represented_country`.
- R7 (A4) A resolved country code that is not ISO 3166-1 alpha-2 returns `country_unsupported`, `allowed` false, `country` set to the resolved object.
- R8 (A5) `country` is `{iso_code, name}` with the English name, or null.
- R9 (A6) Every `detail` sentence ends with a full stop and never contains the IP address.

**Request**
- R10 (A2) Required: `ip_address` (string) and `allowed_countries` (array of strings). Optional: `session_id`, `requested_at`. Required fields are strict.
- R11 (A2, C6) Unknown fields are ignored (names logged at DEBUG only) and never produce a warning. A misspelled required field counts as missing.
- R12 (A10) Each country entry is upper-cased (ASCII) before validation, the duplicate check and matching. Entries are not trimmed.
- R13 (A10) An entry that is not one of the 249 assigned ISO 3166-1 alpha-2 codes (for example `XK`, `UK`, `USA`, `1`, `" DE"`) gives 422. It is never a warning and never ignored.
- R14 (A10) An empty `allowed_countries` gives 422 at `/allowed_countries`. There is no deny-all or allow-all default.
- R15 (A10) A duplicate entry (compared after upper-casing) gives 200 with a `duplicate_country` warning for each repeat after the first. The decision is unchanged; the list is a set.
- R16 (A9, B EC-17) `ip_address` is parsed with `net/netip`; a zone is dropped (`WithZone("")`), not an error; an IPv4-mapped IPv6 address is unmapped and treated as IPv4. A string `netip.ParseAddr` rejects gives 422 (A16).

**Optional fields**
- R17 (A12, C3) `session_id` is at most 128 characters (runes). When valid it is echoed back and logged in full on every log line of the request.
- R18 (A12) `session_id` of `""` counts as absent: nothing echoed, no warning. An absent or invalid `session_id` is omitted from the response, never null.
- R19 (A12) A `session_id` that is too long or not a string, or a `requested_at` that is not valid RFC 3339 or not a string, is ignored. The request is not rejected; the 200 response carries the warning `invalid_session_id` or `invalid_requested_at`; the WARN log keeps only the length (or the JSON type when the value is not a string).
- R20 (A2) `requested_at` accepts any valid RFC 3339 timestamp, with any offset and fractional seconds. It is only logged, never returned, and changes no behaviour.

**Warnings**
- R21 (A7) `warnings` is an array of `{code, detail, pointer?}`; it is omitted when empty, never `[]`; the code list is open and clients ignore unknown codes.
- R22 (A7) Pointers: `/allowed_countries/<index>` for each repeated entry after the first (`["IT","DE","IT"]` gives `/allowed_countries/2`); `/session_id`; `/requested_at`.
- R23 (A7) Warning `detail` texts: `'IT' appears more than once in allowed_countries.` (normalised code), `session_id must be a string of at most 128 characters; the value was ignored.`, `requested_at must be a valid RFC 3339 timestamp; the value was ignored.`

**Errors**
- R24 (A8) Errors are RFC 9457 `application/problem+json` with `type`, `title`, `status`, `detail`; `type` is the configured base URL plus the problem name.
- R25 (A8) Statuses: invalid JSON 400 `malformed-request`; valid JSON that is not an object 422 `invalid-request`; wrong or missing Content-Type 415 `unsupported-media-type`; body over the cap 413 `request-too-large`; method not allowed 405 `method-not-allowed`; no database loaded 503 `database-unavailable`; unexpected failure or panic 500 `internal-error` with no internals in the body. Unknown paths use the standard library 404.
- R26 (A8, A9) Only `invalid-request` has `invalid_params: [{pointer, reason}]`, without codes. Its top-level `detail` is always "The request body is missing required fields or has invalid values."
- R27 (A9) All problems of a request are listed in one 422: `ip_address` first, then `allowed_countries` problems by ascending index. An offending entry is quoted as the caller sent it; the IP is never quoted.
- R28 (A11, H1) The body cap is 16,384 bytes (configurable, `MAX_REQUEST_BODY_BYTES`). Exactly 16,384 is accepted; 16,385 gets 413 with "The request body exceeds the maximum allowed size of 16384 bytes." An invalid cap value stops startup with an error naming the variable.
- R29 (D1) The client treats any 4xx or 5xx as "error".

**Pipeline order (H4)**
- R30 (H4) Order: recovery, path, method, Content-Type, body size, JSON, required fields, database loaded (else 503), special-range check (no lookup), lookup, decision.

**Database and reload**
- R31 (A13) The only third-party dependency is `github.com/oschwald/maxminddb-golang/v2`; decode only `country.iso_code` and `country.names.en`. No performance number is stated anywhere until it has been measured.
- R32 (D2) The service polls the database file's modification time only. A new file is swapped in only if it opens, is a Country database, has a newer build date than the loaded one, and answers the sanity lookups as expected. Otherwise the old data is kept and an ERROR is logged. In-flight requests are never dropped. The first load attempt runs at startup and then on every interval.
- R33 (D3) The sanity lookups are a setting supplied by the developer (known IP and country pairs), checked on every load including the first, with no built-in default. If the setting is empty or unset, the other checks still run, one WARN says sanity lookups are off, and startup is never stopped by it.
- R34 (D4) The service starts without data. While no database is loaded, checks and `/readyz` return 503 `database-unavailable`, and `/livez` returns 200.
- R35 (D11, J7) The service never downloads the database and never touches the licence key; the update sidecar writes it into a shared volume that the service mounts read-only. Credentials are never in env values, the image or the repository.

**Logging**
- R36 (C2) No log line writes an IP address and no response contains it. If an address type is logged by mistake it renders only the prefix: /24 for IPv4, /48 for IPv6.
- R37 (C1, C4) One JSON summary line per request via `log/slog`. A `country_unknown` result logs a WARN with `address_category` (`private`, `loopback`, `link_local`, `unspecified`, `reserved`, `public_unmapped`), never the IP.
- R38 (C8) A panic is logged at ERROR with the stack and with `session_id` only if already known; otherwise the field is omitted.

**Operational endpoints**
- R39 (K1) `/livez` returns 200 `{"status":"ok"}` whether or not the service is ready. There is no `/healthz`.
- R40 (K2) `/readyz` returns 200 with `status`, `database_type` and `database_built_at` when a database is loaded, otherwise 503 `database-unavailable`.
- R41 (K3) `/version` returns `{"version":"<service version>","database_built_at":"<RFC 3339>"}`; `database_built_at` is omitted while no database is loaded.
- R42 (K4) `/metrics` is optional and is cut first. Any other method on these endpoints returns 405 with `Allow: GET, HEAD`.

## Layout

Go server-project layout (A14): `cmd/geoip-service` (wiring only), `internal/config`, `internal/iso`, `internal/netclass`, `internal/request`, `internal/decision`, `internal/geoipdb`, `internal/reload`, `internal/httpapi`, `tests/scenarios`. IP parsing and normalisation live in `netclass`; `decision` gets an already parsed address and does not know HTTP. Unit tests sit next to the code.

## Acceptance example

Request: `{"ip_address":"81.2.69.160","allowed_countries":["GB"]}` against `testdata/GeoIP2-Country-Test.mmdb`.
Response: 200 `{"allowed":true,"reason":"country_allowed","country":{"iso_code":"GB","name":"United Kingdom"},"detail":"The IP address is located in GB (United Kingdom), which is in allowed_countries."}`.
The same address with `["DE"]` gives `allowed` false, `reason` `country_not_allowed`.

## Out of scope

VPN and proxy detection; storing any state; non-ISO country codes (so `XK` can never be allowed); building gRPC (designed in the notes, F2); using the real GeoLite2 file in any test.
