# API design notes

Reasons and sources for every decision of geo-ip-allowlist-service. If code or contract disagree with these notes, the notes win. Change order: notes first, then `api/`.

Status: **A** approved by the developer (source ID points to the appendix in `decision-log.md`); **P** proposed by the designer (every P item is listed in G, in `decision-log.md`); **U** unverified.

**Plan approval.** The developer approved the plan and all proposed items G2 to G47 as written in `decision-log.md`, with one exception: the sanity-lookups setting must never make startup fail. If it is empty or unset, the service logs a WARN and starts (this replaces the earlier G35, see G35 and D3). Every item marked P below is therefore approved (source: PLAN-APPROVAL in the Appendix). **G1 (ISO check date)** was supplied by the developer later in the session and is recorded in G1. BQ2 and BQ3 stay open business questions with their defaults.

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

**A9 `invalid_params`.** (Exception recorded in G76: an entry in `allowed_countries` is the caller's own input and is quoted back even if it looks like an IP address.) Items `{pointer, reason}`, no codes (A, P3). Order: `ip_address` first, then the `allowed_countries` problems by ascending index; errors on different fields appear together in one 422 and every error is reported (A, R2, R2-a). The entry in a reason is quoted as the caller sent it; the IP is never quoted (A, P3, E2a). Reason texts (P, G7):
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

**A13 Library.** One third-party dependency: `github.com/oschwald/maxminddb-golang/v2`; decode only `country.iso_code` and `country.names.en`. Reason: the Go standard library cannot read the MaxMind `.mmdb` format; according to its own documentation the library does fast IP lookups in-process from a single file, with no network call and no extra service. The file is opened once at startup and swapped when a new version arrives. Rejected: `geoip2-golang`, writing our own reader (few dependencies). The speed claim was benchmarked in S-006; the measured numbers are in D7. A. Source: E1.

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
| EC-29 | Unknown fields | ignored; count at DEBUG, no names | A (E2e); G75 |
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
- **C6** Unknown fields: DEBUG, the count only (`unknown_fields_count`); names are not logged (G75, replaces the earlier "names, at most 20" rule of G8). Never in `warnings` (A).
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
- **E11** A database file that is damaged inside, with its metadata intact, is accepted at startup or reload: `/readyz` returns 200 and checks return 500. The file is not verified with the library's `Reader.Verify()`. The update tool checks what it downloads, and the sanity lookups (when set) catch most damage. No wrong "allowed" answer was found. Possible fix: call `Verify()` in `geoipdb.OpenBytes` and test it with a damaged copy of the test database.
- **E12** `PROBLEM_BASE_URL` with a query or fragment gives odd problem `type` values, because the problem name is appended to the text. Use a plain path prefix.
- **E13** Invalid UTF-8 inside a JSON string is replaced by the Unicode replacement character, not rejected.

---

## F. Future expansion

- **F1** A separate reason code per cause, only when a client needs to react differently to each one (A, E2a).
- **F2** gRPC (designed, not built; A, E8). Design (P, G23): service `geoip.v1.IpCheckService`, `rpc CreateIpCheck(CreateIpCheckRequest) returns (IpCheck)`, mirroring the HTTP fields and reason strings. Status mapping: 400 and 422 to `INVALID_ARGUMENT` with field violations (pointer, reason); 413 to `RESOURCE_EXHAUSTED`; 503 to `UNAVAILABLE`; 500 to `INTERNAL`. Building it would need approval for `google.golang.org/grpc`, `google.golang.org/protobuf` and a code generator.
- **F3** VPN detection.
- **F4** Support for codes outside ISO 3166-1 alpha-2.

---

## G. Open items and decision log

Moved to [decision-log.md](decision-log.md): the open items (G1 onward, with the proposed and decided entries) and the appendix with the question-and-answer log (IDs such as E2d, R1, GR1). Items cited in this file as `Gn` or by a question ID are there.

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

- **Section lettering** A core decisions, B corner cases, C logging, D gateway policy and operations (D11 how the database reaches each pod), E known limitations, F future, G open items, H implementation notes, I unused (AI usage is in `docs/ai-usage.md`), J security (J7 the secret for the update job), K operational endpoints. 
