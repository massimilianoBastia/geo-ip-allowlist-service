# API design notes

Reasons and sources for every decision of geo-ip-allowlist-service. If code, contract or brief disagree with these notes, the notes win. Change order: notes first, then `api/`, then `docs/agent-design-brief.md`.

Status: **A** approved by the developer (source ID points to the Appendix); **P** proposed by the designer (every P item is listed in G); **U** unverified.

**Plan approval.** The developer approved the plan and all proposed items G2 to G47 as written in this file, with one exception: the sanity-lookups setting must never make startup fail. If it is empty or unset, the service logs a WARN and starts (this replaces the earlier G35, see G35 and D3). Every item marked P below is therefore approved (source: PLAN-APPROVAL in the Appendix). **G1 (ISO check date)** was supplied by the developer later in the session and is recorded in G1. BQ2 and BQ3 stay open business questions with their defaults.

---

## A. Core design decisions

**A1 Endpoint.** `POST /v1/ip-checks`, JSON body. A. Reason: RFC 9110: when the input travels in a body and nothing is stored, POST is the right method; it keeps the IP out of access-log URLs; `/v1` follows the Google AIP guidelines and keeps the URL clear when debugging. Rejected: `GET` with a query string, `/v1/check`. Source: E2.

**A2 Request fields.** `ip_address` (string, required), `allowed_countries` (array of strings, required, at least one entry). Optional: `session_id` (string, at most 128 characters, counted as runes; callers should use a UUID, not enforced; control characters are not checked because JSON logs escape them) and `requested_at` (RFC 3339; any offset and fractional seconds accepted; only logged; never returned; changes no behaviour; stale requests are not rejected). Unknown fields are ignored, never rejected, never in `warnings`; a misspelled required field counts as missing. Required fields are decoded strictly; only the two optional fields are decoded leniently. A. Source: E2, E2d, E2e, E2d-3, E2d-4, E2d-6.

**A3 Response.** HTTP 200 whenever the check completes. Fields: `allowed` (boolean, true exactly when `reason` is `country_allowed`), `reason`, `detail`, `country`, `session_id` (only when valid), `warnings` (only when not empty). A. Reason: the client has three states: allowed, not allowed, error; a "not allowed" result is a successful check. Source: E2, E2c, E2d-5.

**A4 Reason codes.** Exactly four: `country_allowed`, `country_not_allowed`, `country_unknown`, `country_unsupported`. Clients switch on `reason` and never parse `detail`. A. Source: E2.
- `country_unknown`: every case in which no country is found (private, loopback, link-local, unspecified, multicast and other reserved ranges; a public IP missing from the data; a record without a country). Response: allowed false, `country` null. Reason: the request is valid and the check completed, there is just no country to match; failing open would let anyone bypass the restriction. Source: E2a.
- `country_unsupported`: the IP resolved to a country code that is not an ISO 3166-1 alpha-2 code (today only `XK` in the real GeoLite2 file). Response: allowed false, `country` is the resolved object, for example `{"iso_code":"XK","name":"Kosovo"}`. Reason: the contract is the ISO standard, so no allowed list can contain such a code; these IPs are always blocked. Source: E2b.
- Never fall back to `registered_country` or `represented_country`: a fallback could let a restricted user through. Source: E2a (B2).

**A5 `country` object.** `{iso_code, name}`, or `null`. `name` is the English name from the database (`names.en`); it is informational and never used for logic. If the database has the country code but no English name, `name` is `""`, so the object always has both members. A. Source: E2, CR3.

**A6 `detail` texts.** Every sentence ends with a full stop (A, R1-a). Never contains the IP (A, E2a).

| Case | Text | Status |
|---|---|---|
| private | "The IP address is in a private range and has no associated country, so it is blocked." | A (E2a) |
| loopback / link-local (one shared sentence) | "The IP address is a loopback or link-local address and has no associated country, so it is blocked." | A (E2a) |
| public_unmapped | "No country is recorded for this IP address in the geolocation database, so it is blocked." | A (E2a) |
| unspecified | "The IP address is the unspecified address and has no associated country, so it is blocked." | P (G3) |
| reserved (multicast, carrier-grade NAT, documentation, other) | "The IP address is in a reserved range and has no associated country, so it is blocked." | P (G3) |
| unsupported | "The IP address is located in a country whose code is not an ISO 3166-1 alpha-2 code, so it cannot be in any allowed list and is blocked." | P (G3) |
| allowed | "The IP address is located in GB (United Kingdom), which is in allowed_countries." | P (G3) |
| not allowed | "The IP address is located in GB (United Kingdom), which is not in allowed_countries, so it is blocked." | P (G3) |

A record found without a country uses the public_unmapped sentence (P, G3, G4).

**A7 Warnings.** `warnings` is an array of objects `{code, detail, pointer?}`; `pointer` only when it applies; the field is omitted when empty, never `[]`. The code list is open: clients ignore codes they do not know. A. Source: E2d-1.

| Code | Pointer | Detail | Status |
|---|---|---|---|
| `duplicate_country` | `/allowed_countries/<index>` of each repeated entry after the first | "'IT' appears more than once in allowed_countries." (normalised code) | A (W2, W3, W3-a) |
| `invalid_session_id` | `/session_id` | "session_id must be a string of at most 128 characters; the value was ignored." | A (W3) |
| `invalid_requested_at` | `/requested_at` | "requested_at must be a valid RFC 3339 timestamp; the value was ignored." | A (W3) |

Reason for the pointers: the caller must be informed, there could be a bug on their side (W2). Duplicates never change the decision; the list is a set (W4). Order of warnings in the array: duplicates by index, then `/session_id`, then `/requested_at` (P, G29).

**A8 Problems (RFC 9457 `application/problem+json`).** Fields `type`, `title`, `status`, `detail`. `type` is the configured base URL plus the name (absolute URL; default base `https://geo-service.example/problems/`). Only `invalid-request` carries `invalid_params`. A. Source: R1, P1, P2, P3, P4, E3.

| Name | Status | Title | Case | Detail | Detail status |
|---|---|---|---|---|---|
| `malformed-request` | 400 | Malformed request | body is not valid JSON | "The request body is not valid JSON." | P (G11) |
| `invalid-request` | 422 | Invalid request | JSON is not an object, or required fields missing or invalid, or empty list | "The request body is missing required fields or has invalid values." | A (P4) |
| `request-too-large` | 413 | Request too large | body above the cap | "The request body exceeds the maximum allowed size of 16384 bytes." | A (R1) |
| `unsupported-media-type` | 415 | Unsupported media type | Content-Type is not `application/json` | "The request Content-Type must be application/json." | P (G11) |
| `method-not-allowed` | 405 | Method not allowed | not POST; response has `Allow: POST` | "This resource only supports POST." (on the operational endpoints: "This resource only supports GET and HEAD.") | P (G11) |
| `database-unavailable` | 503 | Service unavailable | no database loaded | "The geolocation database is not loaded yet; retry later." | P (G11) |
| `internal-error` | 500 | Internal error | unexpected failure or panic | "An unexpected error occurred." | P (G11) |

`request-too-large` is the name after the base in the `type` URL, not a code field; hyphenated like a URL path; snake_case applies to JSON member names (R1). Unknown paths use the standard library 404 (P, G10). Any 4xx or 5xx is the client's "error" state (A, E3).

**A9 `invalid_params`.** Items `{pointer, reason}`, no codes (A, P3). Order: `ip_address` first, then the `allowed_countries` problems by ascending index; errors on different fields appear together in one 422 and every error is reported (A, R2, R2-a). The entry in a reason is quoted as the caller sent it; the IP is never quoted (A, P3, E2a). Reason texts (P, G7):
- "ip_address is required." (missing, null, misspelled)
- "ip_address must be a string."
- "ip_address must be a valid IPv4 or IPv6 address." (P, G48; "without a zone" removed because a zone is no longer an error, A16)
- "allowed_countries is required."
- "allowed_countries must be an array of strings."
- "allowed_countries must contain at least one country code."
- "Each entry in allowed_countries must be a string."
- "'USA' is not an ISO 3166-1 alpha-2 country code."
- "The request body must be a JSON object." (pointer `""`)

**A10 Country codes.** Each entry is upper-cased (ASCII only, P G30) before validation, the duplicate check and matching (A, E4-a). No trimming: `" DE"` is not two ASCII letters and gives 422 (A, E4-c). An entry that is not a valid code (`XK`, `USA`, `1`, `UK`) gives 422 with a pointer; it is never a warning and never ignored, because a silent drop would block that country's users with no trace; all invalid entries are listed (A, E4-b). A duplicate gives 200 with a warning, also after case normalisation (`["DE","de"]`) (A, W1). An empty list gives 422 at `/allowed_countries` (A, E2e-4). Valid codes are the constant list of the 249 officially assigned ISO 3166-1 alpha-2 codes; it excludes exceptionally reserved codes (`UK`, `EU`, ...) and user-assigned codes (`XK`, `AA`, `QM`-`QZ`, `XA`-`XZ`, `ZZ`); it is pinned by a test written before the list (exactly 249 entries, two upper-case ASCII letters, no duplicates, `GB` and `IT` present, `UK` and `XK` absent, no file read). The same list validates requests and detects `country_unsupported` (A, E2b-1, L1). Sources: ISO 3166-1 on ISO's Online Browsing Platform (https://www.iso.org/obp/ui/#search) and the ISO glossary for ISO 3166 (https://www.iso.org/glossary-for-iso-3166.html); tzdata `iso3166.tab` (249 entries) only as a secondary cross-check. ISO has no version number, so the check date is cited and kept in a comment next to the list (G1).

**A11 Body cap.** 16,384 bytes (16 KiB), configurable (`MAX_REQUEST_BODY_BYTES`). Exactly 16,384 bytes gets the normal answer, 16,385 gets 413. It bounds the list; there is no separate maximum list length. A valid list has at most 249 distinct entries; extras are duplicates (warning) or invalid (422). A. Source: E4-d, R1, R1-b, R1-c.

**A12 Valid / invalid optional fields.** `session_id` of 129 runes or not a string, `requested_at` invalid or not a string: request not rejected, value ignored, 200 with the warning, WARN log with the length (or the JSON type if not a string). `session_id` `""` counts as absent: nothing echoed, no warning. A absent or invalid `session_id` is omitted from the response, never `null`. Reason: these fields do not change the decision and the caller can fix them. Because the service accepts bad values, the OpenAPI request schema carries no hard type, `maxLength` or `format` constraint for these two fields (a schema-validating client would otherwise reject requests the service accepts); the limits are stated in the descriptions (P, G47). A. Source: E2d, E2d-2, E2d-5.

**A13 Library.** One third-party dependency: `github.com/oschwald/maxminddb-golang/v2`; decode only `country.iso_code` and `country.names.en`. Reason: the Go standard library cannot read the MaxMind `.mmdb` format; according to its own documentation the library does fast IP lookups in-process from a single file, with no network call and no extra service. The file is opened once at startup and swapped when a new version arrives. Rejected: `geoip2-golang`, writing our own reader (few dependencies). The speed claim is unverified and benchmarked in S-006; no number is stated anywhere until measured (U, G2). A. Source: E1.

**A14 Layout.** Go server-project layout (https://go.dev/doc/modules/layout#server-project): `cmd/geoip-service` (wiring only), `internal/config`, `internal/iso`, `internal/netclass` (parsing, normalisation, category, truncated-address log type), `internal/request`, `internal/decision` (pure, no HTTP), `internal/geoipdb`, `internal/reload`, `internal/httpapi` (handlers, problems, operational endpoints, body limit, recovery middleware), `tests/scenarios` (acceptance tests with the test database). Unit tests sit next to the code. IP parsing and normalisation live in `netclass`; `request` calls it; `decision` receives an already parsed address. A. Source: GR1.

**A15 Error precedence.** Checks run in this order and each stops at the first failure: method (405), then Content-Type (415), then body size (413), then JSON syntax (400). Field validation then gives one 422 that lists every problem, in fixed order: `ip_address` first, then the `allowed_countries` entries by ascending index (A9). The database lookup comes after validation, so an invalid request gets 422 even when no database is loaded. 503 applies only to a valid request. This follows the pipeline order in H4 (the steps before the method check, recovery and path, are unchanged). A. Source: CR1.

**A16 `ip_address` syntax.** A string holding an IPv4 or IPv6 text address, parsed with `net/netip` (`netip.ParseAddr`). Anything `netip.ParseAddr` rejects (for example `999.1.1.1`, `010.0.0.1` with a leading zero, `" 1.2.3.4"` with a space, `""`) gives a 422 with pointer `/ip_address`. An IPv4-mapped IPv6 address is unmapped to IPv4 before lookup (G39). The reason text never repeats the IP (A9). A zone is not an error: `netip.ParseAddr` accepts it, and the service drops it with `WithZone("")` before the lookup (then unmaps IPv4-mapped IPv6). `fe80::1%eth0` becomes `fe80::1`, which is link-local, so the answer is 200, `allowed: false`, `country_unknown`. Reason: the zone only names a local network interface and has nothing to do with the country; rejecting a valid address would turn a normal request into an error for no benefit. This replaces the earlier rule that a zone gives 422. A. Source: CR2.

---

## B. Corner cases handled

Each case has a test (see `docs/design/plan.md`, edge cases EC-01 to EC-42). Decisions and their sources:

| ID | Case | Result | Source |
|---|---|---|---|
| EC-01 | Body not valid JSON (empty, text, trailing data) | 400 `malformed-request` | A (E3); detail P |
| EC-02 | Valid JSON that is not an object | 422 `invalid-request`, pointer `""` | A (E3); wording P |
| EC-03 | Content-Type missing or not `application/json` | 415; charset parameter accepted | A (E3); missing/charset P |
| EC-04 | Method other than POST | 405 + `Allow: POST` | A (E3) |
| EC-05 | 16,384 vs 16,385 bytes | 200 vs 413 | A (R1-c) |
| EC-06..09 | `ip_address` / `allowed_countries` missing, null, wrong type, invalid IP | 422 + pointer | A (E2e, E2e-4); invalid IP string P |
| EC-10 | Empty list | 422 | A (E2e-4) |
| EC-11, EC-12 | Non-string / non-ISO entries | 422, all listed, quoted as sent | A (E4-b, P3) |
| EC-13 | Several errors | one 422, ordered | A (R2, R2-a) |
| EC-14, EC-15 | Lowercase / duplicates | upper-cased / warning | A (E4-a, W1) |
| EC-16, EC-17 | IPv6; IPv4-mapped IPv6; IPv6 with a zone | looked up; unmapped first; zone dropped (`fe80::1%eth0` is looked up as `fe80::1`) | A for resolving IPv6 and the zone (A16); unmapping P (G39) |
| EC-18..24 | Private, loopback, link-local, unspecified, reserved, missing, no country | `country_unknown` | A (E2a) |
| EC-25 | `registered_country` differs | uses `country` only | A (E2a) |
| EC-26 | Non-ISO resolved code | `country_unsupported` | A (E2b) |
| EC-27, EC-28 | `session_id`, `requested_at` edge values | see A12 | A (E2d) |
| EC-29 | Unknown fields | ignored; names at DEBUG | A (E2e); logging format P |
| EC-30 | No database at startup | 503 on checks and `/readyz`; `/livez` 200; `/version` 200 | P (G16, G32) |
| EC-31..33 | Bad, older or removed database file | old data kept, ERROR | A (E6) |
| EC-34 | Invalid configuration | startup stops, error names the variable | A (R1-b) for the body cap; validation of the other settings P (G42) |
| EC-35 | Lookup error | 500 `internal-error` | A (E3) |
| EC-36 | Unknown path | standard library 404 | P (G10) |
| EC-37 | IP in logs/responses | never | A (E2a, GR logging) |
| EC-38 | SIGTERM during a request | request completes | P (G40) |
| EC-39 | `ıt` (dotless i) | 422, not `IT` | P (G30) |
| EC-40 | Panic | 500, no internals, ERROR with stack | A (GR ops) |
| EC-41 | Guard type rendering | /24, /48 | A prefix lengths; strings P (G34) |
| EC-42 | Empty sanity setting | other checks run; one WARN | A (GR5) |

---

## C. Logging policy

JSON via `log/slog` on stdout (P, G12). No secrets and no IP (CLAUDE.md).
- **C1** One summary line per request: `method`, `path`, `status`, `duration_ms`, `reason`, `allowed`, `country_iso`, `allowed_countries_count`, `requested_at` (when valid), `session_id` (when valid). Levels: WARN for `country_unknown` (A, E2a); INFO for allowed and not allowed, WARN for `country_unsupported`, INFO for 4xx, ERROR for 5xx (P, G5).
- **C2** The IP is never written to a log line and the response never contains it (A, E2a, GR logging). The truncated-address type in `netclass` is only a safety guard if a line ever logs an address by mistake: it renders only the prefix, /24 for IPv4 and /48 for IPv6 (A); exact string format P (G34). Two tests are kept: no full IP in logs, and only the prefix when the type is used.
- **C3** A valid `session_id` is logged in full on every line of its request; it is the correlation key and not treated as sensitive (A, E2d).
- **C4** `address_category` (`private`, `loopback`, `link_local`, `unspecified`, `reserved`, `public_unmapped`) is logged on the WARN line of `country_unknown`, never the IP; multicast, carrier-grade NAT and documentation ranges are `reserved` (A, E2a). Scope limited to `country_unknown` lines, and `public_unmapped` for records without a country: P (G4, G5).
- **C5** Invalid optional field: WARN with the length (`session_id_length`, `requested_at_length`), or the JSON type if not a string; snake_case names (A, E2d-2). The `*_type` field names are P (G41).
- **C6** Unknown fields: DEBUG, names only, at most 20, plus a count (P, G8). Never in `warnings` (A).
- **C7** Database events: INFO on load with `database_type` and `database_built_at`; ERROR when a file is rejected or removed (A, E6); WARN once while waiting for the first file (P, G16); WARN once when sanity lookups are off (A, GR5).
- **C8** A panic is logged at ERROR with the stack and `session_id`; before the session id is known the field is simply omitted, no value invented (A, GR ops).

---

## D. Side notes on gateway policy and operations

- **D1** The client has three states: allowed, not allowed, error. Any 4xx or 5xx is "error". Clients ignore unknown warning codes. A (E2c, E3, E2d-1).
- **D2** Reload. The service polls the file's modification time. Before a swap the file must open, be a Country database, have a build date newer than the loaded one, and give the expected answers to the sanity lookups. On failure the old data is kept and an ERROR is logged. In-flight requests are never dropped. A (E6). The first load attempt runs at startup, before any tick, and then again on every interval (P, G44).
- **D3** Sanity lookups are a setting supplied by the developer: a few known IP and country pairs, checked on every reload and on the first load. There is no built-in default. If the setting is empty, the other checks still run and the service logs a WARN that sanity lookups are off. No manual check with real credentials, because tests never use the real file. An empty or unset setting never stops startup (A, PLAN-APPROVAL). A (GR5). Name `DATABASE_SANITY_CHECKS` and format `ip=CC,ip=CC`: P (G14).
- **D4** The service starts without data, `/readyz` and checks return 503 until the first load, `/livez` is always 200. P (G16): the sidecar needs time for its first download and exiting would cause a restart loop.
- **D5** Data freshness: default keep answering and expose the build date in `/readyz` and `/version`; the sidecar updates every 24 hours. Open business question BQ2 (G, below).
- **D6** Scope and cut order. Core: the service, in-process reload, `/version`, the Dockerfile, the Kubernetes manifest. gRPC is designed but not built. The only optional last step is `/metrics`; if time runs short cut from the end: `/metrics` first, then the optional `docker-compose.yml`. Reload, Dockerfile and Kubernetes manifest are never cut. A (E8).
- **D7 Storage choice and lookup speed.** The country data is an in-process `.mmdb` file read into memory, not Redis or another service: the library looks IPs up in-process from a single file, with no network call and no extra service (A13, KD5). Lookup numbers from the S-006 benchmark (Go benchmarks in `internal/geoipdb/bench_test.go`). They come from one run on the synthetic test database (`testdata/GeoIP2-Country-Test.mmdb`), not the real GeoLite2 file, and there is no threshold. CPU AMD Ryzen 7 PRO 8840HS, Go 1.26.10 linux/amd64, `maxminddb-golang/v2` v2.9.0, 2026-11-08. Each operation is a lookup plus decoding `country.iso_code` and `country.names` into a struct:
  - IPv4 hit `81.2.69.160`: 765.6 ns/op, 392 B/op, 5 allocs/op
  - IPv6 hit `2001:218::1`: 769.2 ns/op, 392 B/op, 5 allocs/op
  - miss `8.8.8.8`: 23.08 ns/op, 0 B/op, 0 allocs/op
  - parallel (16 CPUs, IPv4 hit): 156.7 ns/op, 392 B/op, 5 allocs/op
  Added by the developer's decision, 2026-11-08 (G55). D8 to D10 are not used; D11 keeps its number.
- **D11 How the database reaches each pod.** The update sidecar (official `ghcr.io/maxmind/geoipupdate`) downloads `GeoLite2-Country.mmdb` into an `emptyDir` volume mounted at `/usr/share/GeoIP`. The service container mounts the same volume read-only, polls the modification time, verifies the file and swaps it in. The service never downloads the file and never touches the licence key. A for the mechanism (CLAUDE.md). Edition `GeoLite2-Country`, `GEOIPUPDATE_FREQUENCY=24`, image choice: P (G18).

---

## E. Known limitations

- **E1** VPNs and proxies are not detected (out of scope).
- **E2** `XK` (Kosovo) can never be allowed: the contract is ISO 3166-1 alpha-2 (E2b).
- **E3** With duplicate JSON keys the last one wins.
- **E4** A file rewritten without changing its modification time is not noticed until the next change.
- **E5** 6to4, Teredo and NAT64 addresses are not unwrapped.
- **E6** The benchmark runs against the small test database, not the real one.
- **E7** The IP supplied by the gateway is trusted.

---

## F. Future expansion

- **F1** A separate reason code per cause, only when a client needs to react differently to each one (A, E2a).
- **F2** gRPC (designed, not built; A, E8). Design (P, G23): service `geoip.v1.IpCheckService`, `rpc CreateIpCheck(CreateIpCheckRequest) returns (IpCheck)`, mirroring the HTTP fields and reason strings. Status mapping: 400 and 422 to `INVALID_ARGUMENT` with field violations (pointer, reason); 413 to `RESOURCE_EXHAUSTED`; 503 to `UNAVAILABLE`; 500 to `INTERNAL`. Building it would need approval for `google.golang.org/grpc`, `google.golang.org/protobuf` and a code generator.
- **F3** VPN detection.
- **F4** Support for codes outside ISO 3166-1 alpha-2.

---

## G. Open items

- **G1** ISO check date: `IT` and `UK` were checked on ISO's Online Browsing Platform on 8 November 2026. The ISO glossary for ISO 3166 was read on 8 November 2026: it says `UK` is exceptionally reserved, and the range `XA` to `XZ` is user-assigned, which includes `XK`.
- **G2** Lookup speed: measured by the S-006 benchmark; the numbers, with CPU, Go version and date, are in D7 only.
- **G3** `detail` texts for unspecified, reserved, unsupported, allowed, not allowed; reuse of the public_unmapped sentence for records without a country.
- **G4** `public_unmapped` as category for records without a country.
- **G5** Log levels and scope of `address_category`.
- **G6** Range lists of the address classifier.
- **G7** `invalid_params` reason texts; root pointer `""`.
- **G8** Unknown-field logging format (names only, max 20, sorted, count).
- **G9** Missing Content-Type gives 415; charset parameter accepted.
- **G10** Standard library 404 for unknown paths.
- **G11** `detail` texts of the problems 400, 415, 405, 503, 500.
- **G12** Setting names and defaults (H1); env vars and JSON logs.
- **G13** Server timeouts.
- **G14** Sanity setting name and format.
- **G15** Load the file into memory (`OpenBytes`) and swap an atomic pointer.
- **G16** Start not ready; WARN while waiting.
- **G17** 503 placed after request validation.
- **G18** Sidecar image and frequency; `emptyDir`.
- **G19** README generated by a test with an `-update` flag.
- **G20** Transport: plain HTTP plus a NetworkPolicy.
- **G21** The benchmark reports numbers only.
- **G22** Empty `name` when the record has no English name.
- **G23** gRPC design content.
- **G24** `/metrics` content and format.
- **G25** Content-Length checked before reading the body.
- **G26** Base images (`golang:1.26`, distroless static nonroot).
- **G27** Manifest sizing (2 replicas) and path `deploy/kubernetes/`.
- **G28** `/livez` body, and the `/readyz` shape: flat fields `database_type` and `database_built_at` (renamed from `build_time` to match `/version`).
- **G29** Order of warnings.
- **G30** ASCII-only upper-casing.
- **G31** Trailing data after the JSON object gives 400.
- **G32** `/version` with no database loaded omits `database_built_at`.
- **G33** A panic after headers were sent aborts the connection.
- **G34** String rendering of the guard type (masked-prefix `String()`; zero address renders `invalid`). The type also implements `slog.LogValuer` (`LogValue` method) so a log line shows the prefix; `MarshalText` is not used (developer's decision, 2026-11-08).
- **G35** (changed at plan approval) The manifest and the Compose file ship `DATABASE_SANITY_CHECKS` empty, with a comment showing the `ip=CC,ip=CC` format. They carry no placeholder string, because a malformed value is a configuration error. An empty setting never stops startup; it logs one WARN "sanity lookups are off". The developer supplies the real pairs for the real GeoLite2 file.
- **G36** `run` dependencies (`listener`, `wrap`, `version`).
- **G37** Version injected with `-ldflags "-X main.version=<value>"`, default `dev`.
- **G38** Handling of the indirect `go.mod` requirements (no tidy in S-004, tidy in S-005).
- **G39** Unmapping of IPv4-mapped IPv6 addresses before lookup.
- **G40** Graceful shutdown on SIGTERM: in-flight requests complete within `SHUTDOWN_TIMEOUT`.
- **G41** Field names `session_id_type` and `requested_at_type` for the WARN log of a non-string optional value.
- **G42** Validation rules for the settings other than the body cap (positive durations, known log level, absolute `PROBLEM_BASE_URL`, well-formed sanity pairs).
- **G43** Container hardening: non-root, read-only root filesystem, all capabilities dropped, distroless runtime.
- **G44** The first database load attempt runs at startup, before any tick, then on every interval.
- **G45** No test opens a socket: the lifecycle test uses an in-memory listener passed through `deps.listener`; a loopback socket would need your explicit approval because `CLAUDE.md` says "No network in any test".
- **G46** The metrics series `database_reloads_total` is left out (no reload observer hook); add it later only with a tested hook in `internal/reload`.
- **G47** The OpenAPI request schema does not constrain the type, length or format of `session_id` and `requested_at`, because the service accepts bad values with a warning.
- **G48** (proposed) The `invalid_params` reason for an invalid `ip_address` loses "without a zone": "ip_address must be a valid IPv4 or IPv6 address." (consequence of A16). Order inside the parser: drop the zone, then unmap IPv4-mapped IPv6.
- **G49** (decided by the developer, 2026-11-08) The ISO check date is kept only as a comment next to the list in `internal/iso/iso.go`. No test reads the source file to check it, and there is no date constant. The reviewer checks the comment and the list against the sources named in A10.
- **G50** (decided by the developer, 2026-11-08) `maxminddb-golang/v2` v2.9.0 has no `FromBytes`; the in-memory constructor is `OpenBytes(buffer, ...)`. Same behaviour, new name: G15, H3 and the plan (components table, KD13, assumptions) now say `OpenBytes`. `OpenBytes` keeps the slice it is given, so the caller must never modify it after opening.
- **G51** (decided by the developer, 2026-11-08) `netclass.Classify` of the zero (invalid) address returns `reserved`, so an invalid address can never count as public (fail closed).
- **G52** (decided by the developer, 2026-11-08) The codes in `internal/iso/iso.go` were copied from tzdata `iso3166.tab`. They have not been checked code by code against ISO; only `IT` and `UK` were checked on ISO's Online Browsing Platform (G1). The reviewer must check the full list against the sources in A10. The comment next to the list says so.
- **G53** (decided by the developer, 2026-11-08) The `go` line in `go.mod` is `go 1.26.0`, as `go mod tidy` wrote it in S-005, because `maxminddb-golang/v2` v2.9.0 requires Go 1.26.0 and `go 1.26` is lower. `toolchain go1.26.10` is kept. KD20, S-004, H6 and E10 are updated. GR2 (decision log) keeps its original wording. Dependency versions are pinned in `go.mod` and no others are added: `maxminddb-golang/v2` v2.9.0 (direct) and `golang.org/x/sys` v0.49.0 (indirect). `go.sum` also holds hashes for the library's own test-only modules (`testify` v1.12.1, `yaml/v3` v3.0.5), which are not requirements of this module. Network use by `go mod tidy` accepted (G38).
- **G54** (decided by the developer, 2026-11-08) One case was added to the committed `TestClassify` in `internal/netclass/classify_test.go` after approval: `Classify` of the zero address is `reserved` (G51). The test did not cover the new fail-closed branch. Nothing else in the tests changed. Recorded in `docs/ai-usage.md`.
- **G55** (decided by the developer, 2026-11-08) New item D7: storage choice (in-process `.mmdb`, not Redis) and the S-006 lookup numbers. The numbers live only in D7; G2 points to D7. The lettering stays: D is side notes, D11 is how the database reaches each pod, and D8 to D10 stay unused. "Numbers only" (G21, H8) means the numbers are recorded and there is no pass/fail threshold; the benchmark code stays in the repository. The S-001 and S-006 test lines in the plan are reworded to match. The S-001 test line no longer fixes the number of G items: it requires no gaps in the G numbering and every item listed once.
- **G56** (decided by the developer, 2026-11-08) The README table of the test database (S-005) has a `Registered country` column and a row for `5.83.124.1` (network `5.83.124.0/22`, no country, registered country RO), taken from the MaxMind source data. It is the fixture for the key test that there is no fallback to `registered_country` (KD4): a record with no `country` but a `registered_country` must still give `country_unknown`. `TestDatabaseMatchesExpectedFacts` checks the row and the column for every fixture; the S-005 table in the plan is updated. The S-017 scenario that uses `5.83.124.1` is still to be written.
- **G57** (decided by the developer, 2026-11-08) The `Classify` test (S-008) lists reserved ranges beyond those named in C4 (multicast, carrier-grade NAT, documentation): `0.0.0.0/8` (`0.1.2.3`), `192.0.0.0/24`, `198.18.0.0/15`, and `240.0.0.0/4` with `255.255.255.255`. They are also `reserved`, and the zero (invalid) address is `reserved` too (G51).
- **G58** (decided by the developer, 2026-11-08) The text of the `address_category` values (`private`, `loopback`, `link_local`, `unspecified`, `reserved`, `public_unmapped`) must be pinned by a test when the logging step (S-014) is written. `netclass` names the public category `public`; `public_unmapped` is chosen by `decision` for a public address with no country (C4, G4). Until then no test fixes the logged strings.
- **G59** (decided by the developer, 2026-11-08) Requests to `/livez`, `/readyz`, `/version` and `/metrics` write the same summary line as C1 (`method`, `path`, `status`, `duration_ms`) at DEBUG level only, so a probe every few seconds does not fill the log at INFO. Their 405 and 503 answers are logged the same way.
- **G60** (proposed, coder) `session_id` or `requested_at` given as JSON `null` is handled like any other value that is not a string: ignored, with the warning, and the WARN log carries `session_id_type=null` (or `requested_at_type=null`). An empty `requested_at` string is invalid (length 0). Only an empty `session_id` string counts as absent (A12).
- **G61** (decided by the developer, 2026-11-08) The recovery middleware logs the Go type of the panic value (`panic_type`) and the stack, not the panic value itself, because the value could contain personal data such as an IP address. A second panic's log line never carries the session of an earlier request.
- **G62** (decided by the developer, 2026-11-08) `PROBLEM_BASE_URL` must be an absolute URL, otherwise startup stops with an error naming the variable; a missing trailing `/` is added. `LOG_LEVEL` is read with `slog.Level.UnmarshalText`, so `debug`, `info`, `warn`, `error` (any case) are accepted, and so are offsets such as `info+1` (proposed). A trailing comma in `DATABASE_SANITY_CHECKS` and a lower-case country code in a pair are errors (proposed).
- **G63** (proposed, coder) A failure while reading the request body that is not the size limit (for example a broken connection) is answered as `malformed-request` 400. The declared `Content-Length` is checked first (G25); a body without a declared length is cut at the cap by `http.MaxBytesReader`.
- **G64** (proposed, coder) HEAD on `/v1/ip-checks` is a method that is not POST: 405 with `Allow: POST`.
- **G65** (proposed, coder) The reloader logs the reason of a rejection in the attribute `check` (`open`, `type`, `not_newer`, `sanity`), logs "waiting for database file" (WARN, no data yet) or "database file unavailable" (ERROR, data already loaded) once per failure of the stat call, and logs "sanity lookups are off" when it is created. A file that was rejected is not opened again until its modification time changes (D2, E4). A sanity pair fails if the lookup errors or gives another country code than the pair.
- **G66** (proposed, coder) `run` logs its own startup and shutdown error once and returns it; `main` only exits with status 1. The bootstrap logger (INFO, JSON) is used until the settings are read, so that a bad setting still produces one ERROR line. A clean stop logs INFO "server stopped".
- **G67** (proposed, coder) `reload`, `decision` and `httpapi` do not use `*geoipdb.DB`; they use the small interface `geoipdb.Database` (`Info`, `Lookup`) and the data types `geoipdb.Record`, `geoipdb.Info` and `geoipdb.SanityCheck`, so that tests inject fakes. `reload.Opener` and `reload.Statter` are interfaces with real implementations `FileOpener` and `FileStat`. Files added beyond the plan: `internal/httpapi/httpapi.go` (package comment, `New`, `Settings`, `Source`).
- **G68** (proposed, coder) The recovery middleware learns the session id through a pointer kept in the request context (`setSessionID`); the check handler sets it once the body is parsed.
- **G69** (proposed, coder) `/metrics` is built from the standard library (K4, KD21), not from `prometheus/client_golang`. All series are written, with value 0 before the first event. `database_build_timestamp_seconds` is left out while no database is loaded. The media type is `text/plain; version=0.0.4; charset=utf-8`. A 405 on an operational endpoint counts in `http_problems_total`.
- **G70** (proposed, coder) `docs/problems.md` lists the seven problem types and the `invalid_params` reasons; it is kept in step with the code by hand, and the notes win if they disagree.
- **G71** (proposed, coder) Deployment files: the sidecar image is pinned to `ghcr.io/maxmind/geoipupdate:v7.1.0` (the tag was not checked against the registry); both containers run non-root with a read-only root filesystem and all capabilities dropped, the pod uses `fsGroup: 65532` so the sidecar can write the `emptyDir`; requests and limits are small starting values (service 50m CPU, 64Mi/128Mi; sidecar 10m, 32Mi/64Mi); the NetworkPolicy limits ingress only and names the gateway namespace with the placeholder `api-gateway`; the example Secret is no longer in the manifest: it moved to `deploy/kubernetes/secret.example.yaml` and holds placeholder strings only (G73). Compose publishes the port on `127.0.0.1` only.
- **G72** (decided by the developer, 2026-11-08) `country.name` in the response stays exactly as the database gives it, an empty string when there is none (G22, A5). In the `detail` sentence the cleaned form is used: when the name is empty the parentheses are left out ("located in GB, which is in allowed_countries.").
- **G73** (decided by the developer, 2026-11-08) Run files for a reviewer without credentials. (a) The example Secret moved out of `deploy/kubernetes/geoip-service.yaml` into `deploy/kubernetes/secret.example.yaml`, so `kubectl apply` on the manifest can never replace a real Secret with placeholders; the real Secret is created with `kubectl create secret generic maxmind-credentials --from-file=account_id=... --from-file=license_key=...`. (b) New `docker-compose.test.yml` runs the service alone on `testdata/GeoIP2-Country-Test.mmdb`; `docker-compose.yml` stays the one with the update container. (c) The `geoipupdate:v7.1.0` image (tag checked to exist) runs `mkdir -p /tmp/geoipupdate` at start; with `readOnlyRootFilesystem: true` and no writable `/tmp` it exits with `mkdir: can't create directory '/tmp/geoipupdate': Read-only file system` (run with user 65532, all capabilities dropped, Podman `--read-only-tmpfs=false`). The sidecar therefore gets an `emptyDir` mounted at `/tmp`. (c2) The pod also sets `automountServiceAccountToken: false`: the service-account token is mounted at `/var/run/secrets/...` (= `/run/secrets/...`), inside the sidecar's read-only Secret mount, and the sidecar failed to start with `error mounting ... read-only file system`; the service never calls the Kubernetes API. Both fixes were tested on a `kind` v0.33.0 cluster (rootless Podman, `kubectl` v1.37.2): both containers start, and with the test database copied into the shared volume the service answers through `kubectl port-forward`. The download from MaxMind and NetworkPolicy enforcement were not tested. (d) `README.md`, `docs/api.md`, `docs/deployment.md` and a MIT `LICENSE` were added; the README says what was run and what was not.

Business questions (not blocking, defaults apply): **BQ2** Should very old data ever stop the service from answering, and what does the GeoLite2 licence require about update frequency and deleting outdated copies? Who: the MaxMind account holder and legal. Default: keep answering and expose the age. **BQ3** May end users see the `detail` texts (English, they name countries)? Who: product and customer-facing team. Default: as written, for the gateway and support.

---

## H. Implementation notes (for the coder)

- **H1 Settings** (environment variables):

| Variable | Default | Status |
|---|---|---|
| `LISTEN_ADDR` | `:8080` | P |
| `GEOIP_DATABASE_PATH` | `/usr/share/GeoIP/GeoLite2-Country.mmdb` | P |
| `DATABASE_POLL_INTERVAL` | `60s` | P |
| `DATABASE_SANITY_CHECKS` | empty, no built-in pair | rule A (GR5); name and format `ip=CC,ip=CC` P |
| `MAX_REQUEST_BODY_BYTES` | `16384` (invalid value, not a positive integer, stops startup with a clear error) | A (R1-b) |
| `PROBLEM_BASE_URL` | `https://geo-service.example/problems/` (must be absolute) | default A (P1), name P |
| `LOG_LEVEL` | `info` | P |
| `SHUTDOWN_TIMEOUT` | `10s` | P |

- **H2** Server timeouts: read-header 5 s, read 10 s, write 10 s, idle 60 s (P, G13). Graceful shutdown on SIGTERM.
- **H3** Read the file into memory with `OpenBytes` and swap an atomic pointer; never close a reader that may still be in use (P, G15).
- **H4** Pipeline order: recovery, path, method, Content-Type, body size, JSON, required fields, database loaded (503), special range, lookup, decision.
- **H5** Decode into a map of raw values; strict for required fields, lenient for optional ones; ASCII-only upper-casing.
- **H6** `go.mod`: module `github.com/massimilianoBastia/geo-ip-allowlist-service`, `go 1.26.0`, `toolchain go1.26.10` (A, E10, GR2, G53; the committed `go 1.26.10` line is changed). Indirect requirements: P (G38).
- **H7** README generator: a test with an `-update` flag reads the committed `.mmdb` and writes the fixture-network table between `<!-- BEGIN GENERATED -->` and `<!-- END GENERATED -->` in `testdata/README.md`; without the flag it fails when the file differs (A as requirement; mechanism P, G19).
- **H8** Benchmark: Go benchmarks, no network, test database loaded into memory; IPv4 hit, IPv6 hit, miss, parallel; numbers only (P, G21).
- **H9** ISO sources and check date: see A10.
- **H10** Interfaces only where a dependency crosses a boundary: the database lookup, the file stat, the clock and the logger.
- **H11** Service version injected at build time with `-ldflags "-X main.version=<value>"`, default `dev` (P, G37).
- **H12** `run(ctx context.Context, getenv func(string) string, stdout io.Writer, d deps) error` with `deps{listener, wrap, version}`; `main()` only builds the signal context and calls `run` (P, G36).
- **H13** Layout: see A14.

---

## I. AI usage

See `docs/ai-usage.md` and `docs/ai-log/sessions.md`.

---

## J. Security

- **J1** No secrets in the service, code, config or logs.
- **J2** The IP is never logged or echoed; the truncation guard is a safety net only (A, E2a, GR logging).
- **J3** The body cap bounds request size (A).
- **J4** Transport: plain HTTP inside the cluster, plus a NetworkPolicy that admits only the gateway namespace (P, G20).
- **J5** Non-root, read-only root filesystem, all capabilities dropped, distroless runtime (P, G43).
- **J6** The database volume is mounted read-only in the service.
- **J7 The secret for the update job.** The MaxMind account id and licence key live in a Kubernetes `Secret` (Docker Compose `secrets:` for local runs). They are mounted as read-only files into the update container only (`/run/secrets/<name>`, `readOnly: true`) and read through `GEOIPUPDATE_ACCOUNT_ID_FILE` and `GEOIPUPDATE_LICENSE_KEY_FILE`; these variables hold file paths, the secret is inside the file. They are never in environment values, in the image (no `COPY`, `ADD`, `ARG` or `ENV` for credentials) or in the repository; only `secrets/*.example` placeholders are committed. A (CLAUDE.md).

---

## K. Operational endpoints

- **K1** `GET`/`HEAD /livez`: 200 `{"status":"ok"}` whether or not the service is ready. `/livez` replaces `/healthz` because Kubernetes deprecates `healthz`. Endpoint A (GR ops); body P.
- **K2** `GET`/`HEAD /readyz`: 200 `{"status":"ready","database_type":"...","database_built_at":"<RFC 3339 UTC>"}`, or 503 `database-unavailable`. Including the database build date is A (GR ops); field names and flat shape P (G28).
- **K3** `GET`/`HEAD /version`: 200 `{"version":"<service version>","database_built_at":"<RFC 3339 build date of the loaded database>"}`. Shape A (GR ops). With no database loaded, `database_built_at` is omitted (P, G32).
- **K4** (optional, cut first) `GET`/`HEAD /metrics`, Prometheus text format, standard library only: `ip_checks_total{reason}`, `http_problems_total{type}`, `database_build_timestamp_seconds` (P, G24). A reload counter is deliberately left out: it would need an observer hook in `internal/reload` that the plan does not define. Endpoint scope A (E8).
- Any other method on K1 to K4: 405 with `Allow: GET, HEAD`.

---

## Appendix: decision log (questions and answers)

One entry per ID: the question, the developer's answer, the reason, and where it was settled. All answers are the developer's, given in the design session.

- **E1** Which library reads `.mmdb`? `oschwald/maxminddb-golang/v2`, one authorised dependency; decode only `country.iso_code` and the English name; not `geoip2-golang`, not an own reader; the speed claim must be benchmarked before any number is stated. Reason: stdlib cannot read the format; in-process, single file, no extra service; few dependencies. Settled: A13.
- **E2** HTTP contract? `POST /v1/ip-checks`; `ip_address`, `allowed_countries`, optional `session_id`, `requested_at`; `reason` machine code plus `detail` sentence; `country` object or null; always 200 when the check completes. Reason: RFC 9110, Google AIP, VP's "verbose reason". Settled: A1-A5.
- **E2a** Which code for each no-country case? All `country_unknown`, 200, allowed false, country null; `detail` names the cause, never repeats the IP; no fallback to `registered_country`/`represented_country`; WARN log `address_category`; category not in response. Reason: failing open would let anyone bypass the restriction. Settled: A4, A6, C4.
- **E2b** What is `country_unsupported`? The IP resolved to a code that is not ISO 3166-1 alpha-2 (today `XK`); allowed false; `country` is the resolved object. Reason: the contract is the ISO standard. Settled: A4, E2.
- **E2b-1** How are ISO codes known? Embedded constant list of the 249 codes. Settled: A10.
- **E2c** Is there an `allowed` boolean? Yes, true exactly when reason is `country_allowed`; three client states. Settled: A3.
- **E2d (-1 to -6)** `session_id`, `requested_at`, warnings. `warnings` array of `{code, detail, pointer}`, omitted when empty, open code list, clients ignore unknown codes; WARN logs length or JSON type, snake_case names; `requested_at` any RFC 3339; limit 128 counts characters; `""` is absent; absent/invalid omitted, not null; lenient decoding only for these two fields. Settled: A2, A7, A12.
- **E2e** `allowed_countries` shape; unknown fields? Array of strings; missing/null/non-array/non-string entry gives 422 with a pointer; unknown fields ignored and logged at DEBUG only; misspelled required field counts as missing. Settled: A2, A9.
- **E2e-4** Empty list? 422 at `/allowed_countries`; no deny-or-allow default. Settled: A10.
- **W1-W4, W3-a** Duplicates. 200 with a warning, never 422; `["DE","de"]` is a duplicate; one warning per repeated entry after the first with pointer `/allowed_countries/2` for `["IT","DE","IT"]`; detail "'IT' appears more than once in allowed_countries." with normalised code and final full stop; other two details approved; duplicates never change the decision. Settled: A7, A10.
- **E3** Statuses. Invalid JSON 400 `malformed-request`; valid JSON that is not an object 422 `invalid-request`; wrong Content-Type 415; oversized 413; non-POST 405 with `Allow: POST`; no data 503 `database-unavailable`; unexpected failure 500 `internal-error`; any 4xx/5xx is the client's "error" state. Settled: A8.
- **E4-a to E4-d** Country entries. Upper-case first; invalid entry 422 with pointer, never a warning, never ignored, all listed, because a silent drop would block that country's users with no trace; no trimming; no separate maximum, the 16 KiB cap bounds it. Settled: A10, A11.
- **E5** Test data. Only `testdata/GeoIP2-Country-Test.mmdb` is in the repo; the real GeoLite2 file never goes in it; nothing is fetched. Settled: H7, CLAUDE.md.
- **E6** Reload. Poll the modification time; checks before the swap: opens, Country type, newer build date, a few known-IP lookups; on failure keep old data and log ERROR. Settled: D2.
- **E8** Scope. gRPC designed, not built; the only optional last step is `/metrics`, cut order `/metrics` then `docker-compose.yml`; reload, Dockerfile, Kubernetes manifest are core and never cut. Settled: D6, F2.
- **E10** Go and module. go1.26.10 (linux/amd64); `go 1.26.0` with `toolchain go1.26.10` (G53); module `github.com/massimilianoBastia/geo-ip-allowlist-service`. Settled: H6.
- **L1** ISO list. Exactly the 249 officially assigned codes; no exceptionally reserved or user-assigned codes; pinned by a test written before the list; sources ISO OBP and the ISO glossary, tzdata only as cross-check; check date in a comment next to the list. Settled: A10.
- **R1, R1-a, R1-b, R1-c** 413 and body cap. `request-too-large` is the name after the base in the problem `type` URL, hyphenated, not a code field; same problem+json shape; detail states the limit; every detail sentence ends with a full stop; `MAX_REQUEST_BODY_BYTES`, default 16384, invalid value stops startup; exactly 16,384 accepted, 16,385 gets 413. Settled: A8, A11, H1.
- **R2, R2-a** Several errors? All listed together in one 422; fixed order: `ip_address` first, then countries by ascending index; clients match by pointer. Settled: A9.
- **P1-P4** Problem types. Type base is configuration, default `https://geo-service.example/problems/`, absolute URLs; seven types (names, statuses, titles in A8); `invalid_params` items `{pointer, reason}` with no codes; entry quoted as sent; top-level 422 detail "The request body is missing required fields or has invalid values." Settled: A8, A9.
- **Section lettering** A core decisions, B corner cases, C logging, D gateway policy and operations (D11 how the database reaches each pod), E known limitations, F future, G open items, H implementation notes, I AI usage, J security (J7 the secret for the update job), K operational endpoints. Meaning of "agent brief": `docs/agent-design-brief.md`, the rules of the service only; if brief and notes disagree, the notes win. Change order: notes, then `api/`, then the brief.
- **GR1** Layout. Server-project layout with the package set in A14; unit tests next to the code; acceptance tests in the existing `tests/scenarios/`; each plan step says which tests are unit tests and which are scenarios. Settled: A14.
- **GR2** `go.mod`. The developer's decision wins: `go 1.26` with `toolchain go1.26.10`; change the committed line. Settled: H6.
- **GR5** Sanity lookups. A setting the developer supplies, checked on every reload before the swap; empty means a WARN and the other checks still run; no manual check with real credentials. Settled: D3.
- **GR ops** Operational endpoints and recovery. `/livez` replaces `/healthz` (Kubernetes deprecates `healthz`); `/readyz` stays; `/version` is core and returns `{"version","database_built_at"}`, the same build date is in `/readyz`; `/metrics` stays the optional last step; a recovery middleware turns a panic into a 500 `internal-error` with no internals and an ERROR log with the stack and the session id, the session id simply omitted when not yet known. Settled: K1-K4, C8.
- **PLAN-APPROVAL** Final approval. The developer approved the plan and G2 to G47 as proposed, including G28 (`database_built_at` in `/readyz`), G34, G36, G37, G45, G46 (with `database_build_timestamp_seconds` kept in `/metrics`), G47 and the G3 wording. Exception: the sanity-lookups setting must not make startup fail; if empty, the service logs a WARN. Reason: startup must not depend on a value the developer has not yet supplied. G1 (ISO check date) stays open until the developer supplies the date. No code is started in the design session. Settled: all P items, G35, D3.
- **CR1** (contract review, 2026-11-08) Which error wins when several apply? Order: 405, then 415, then 413 and 400; each stops at the first failure; then one 422 listing all problems, `ip_address` first, then countries by ascending index; the lookup comes after validation, so an invalid request gets 422 even with no database; 503 only for a valid request. Settled: A15.
- **CR2** (contract review, 2026-11-08) What is a valid `ip_address`? A string with an IPv4 or IPv6 text address, parsed with `net/netip`; whatever `ParseAddr` rejects is 422 at `/ip_address`; IPv4-mapped IPv6 unmapped to IPv4; the reason text never repeats the IP. Zone: first asked back because it contradicted A9/EC-08 (zone gave 422); the developer then decided a zone is not an error: drop it with `WithZone("")`, then look up; `fe80::1%eth0` becomes `fe80::1`, link-local, 200 `country_unknown`. Reason: the zone only names a local network interface, has nothing to do with the country, and rejecting a valid address would turn a normal request into an error for no benefit. Settled: A16, A9 (reason text, G48).
- **CR3** (contract review, 2026-11-08) What is `country.name`? The English name from the database, informational, never used for logic; `""` when the code exists but has no English name, so the object always has both members. Settled: A5.
- **GR logging** The IP is never logged and the response never contains it; the truncated-address type is only a safety guard (/24 IPv4, /48 IPv6); two tests are kept (no full IP in logs; only the prefix when the type is used). IP parsing and normalisation live in `netclass`; `request` calls it; `decision` receives an already parsed address. Settled: C2, A14.
