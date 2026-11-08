# Design notes

The reasons behind the service, in one page. The wire contract (fields, `detail` and problem texts, examples) is only in [`api/openapi.yaml`](../api/openapi.yaml); how to run and deploy is in [`deployment.md`](deployment.md). If code and these notes disagree, the notes win: change them first, then `api/`.

The earlier long notes, the plan (steps, edge cases EC-01 to EC-42) and the decision log (G1 to G76 and the question log) are in git history at commit `a93d637`: `docs/api-design-notes.md`, `docs/design/plan.md`, `docs/decision-log.md`, `docs/api.md`, `docs/problems.md`. IDs such as `G63` in code comments point there.

## Decisions

- **Endpoint.** `POST /v1/ip-checks` with a JSON body. POST keeps the IP out of access-log URLs and nothing is stored. Rejected: `GET` with a query string.
- **Three client states.** `200 allowed:true`, `200 allowed:false`, and any 4xx/5xx as "error". A blocked IP is a successful check, not an error.
- **Fail closed.** No country found (private, loopback, link-local, unspecified, reserved, not in the data, record without a country) gives `country_unknown`, blocked. `registered_country` and `represented_country` are never used as a fallback: that could let a restricted user in.
- **Four reasons:** `country_allowed`, `country_not_allowed`, `country_unknown`, `country_unsupported`. Clients switch on `reason` and never parse `detail`.
- **`country_unsupported`.** The IP resolved to a code that is not ISO 3166-1 alpha-2 (today only `XK`). No allowed list can contain it, so it is always blocked (limitation below).
- **Request validation.** `ip_address` parsed with `net/netip`; IPv4-mapped IPv6 is unmapped, a zone is dropped. `allowed_countries` entries are upper-cased (ASCII only) and checked against the 249 assigned ISO codes; an invalid entry is a 422, never silently dropped, because a silent drop would block that country's users with no trace. Duplicates are a warning. An empty list is a 422. All problems are reported in one 422, `ip_address` first, then entries by index.
- **Optional fields** (`session_id` up to 128 characters, `requested_at` RFC 3339) never reject a request: a bad value is ignored and reported in `warnings`. Unknown fields are ignored and only counted.
- **Errors.** RFC 9457 `application/problem+json` for every 4xx/5xx the service writes. Check order, stopping at the first failure: method (405), Content-Type (415), body size (413, cap 16384 bytes), JSON (400), fields (422), database loaded (503). Unknown paths get the standard library `text/plain` 404.
- **Library.** One dependency, `github.com/oschwald/maxminddb-golang/v2`, decoding only `country.iso_code` and `country.names.en`. The standard library cannot read `.mmdb`. The file is read into memory and looked up in-process (about 0.8 µs per hit on the test database, one run on an AMD Ryzen 7 PRO 8840HS, Go 1.26.10; the real file was not benchmarked). No Redis or other service.
- **Layout.** Go server-project layout: `cmd/geoip-service` wiring only; `internal/{config,iso,netclass,request,decision,geoipdb,reload,httpapi}`; `tests/scenarios` for acceptance tests with the test database. `request` and `decision` have no HTTP. Interfaces exist only where a dependency crosses a boundary (database, file stat, opener).
- **`/metrics` from the standard library** (three series), not `prometheus/client_golang`: no dependency for three counters. Revisit if histograms are wanted.
- **gRPC: designed, not built.** Service `geoip.v1.IpCheckService`, `rpc CreateIpCheck(CreateIpCheckRequest) returns (IpCheck)`, same fields and reasons as HTTP. Status mapping: 400 and 422 to `INVALID_ARGUMENT` with field violations; 413 to `RESOURCE_EXHAUSTED`; 503 to `UNAVAILABLE`; 500 to `INTERNAL`. It would need approval for `google.golang.org/grpc`, `protobuf` and a code generator. Not built for lack of time, not because it is hard.

## Keeping the data fresh

The service never downloads anything and never sees the MaxMind key. A sidecar (`ghcr.io/maxmind/geoipupdate`, every 24 hours, edition `GeoLite2-Country`) writes the file to a shared `emptyDir`; the service mounts it read-only. The service polls the file's modification time. A new file replaces the data only if it opens, is a Country database, has a newer build date, and passes the configured sanity lookups (`DATABASE_SANITY_CHECKS`, `ip=CC,ip=CC`; empty means a WARN and the other checks still run). Otherwise the old data stays and an ERROR is logged. The swap is an atomic pointer, so requests in flight are never dropped. The service starts without a database: checks and `/readyz` return 503 until the first file loads, `/livez` stays 200, so the sidecar has time to download. Whether very old data should stop answering is an open business question (BQ2); today it keeps answering and shows its age (`database_built_at` in `/readyz` and `/version`, `database_build_timestamp_seconds` in `/metrics`).

## Logging

JSON with `log/slog` on stdout. One summary line per request: method, path, status, `duration_ms`, `reason`, `allowed`, `country_iso`, `allowed_countries_count`, and `session_id`/`requested_at` when valid. WARN for `country_unknown` and `country_unsupported`, INFO for other results and 4xx, ERROR for 5xx. **The IP is never logged or echoed**: unknown-field names (could be an address) and the panic value are not logged either; only counts, the Go panic type and `address_category` are. `netclass` has a truncated-address type (/24, /48) as a safety net. Probes and `/metrics` log at DEBUG only.

## Security

No secrets in code, config, image or logs. The MaxMind account id and key are read-only mounted files in the sidecar only, read via `GEOIPUPDATE_ACCOUNT_ID_FILE` and `GEOIPUPDATE_LICENSE_KEY_FILE` (paths, not secrets). The service runs non-root with a read-only root filesystem, no capabilities and a distroless image. Plain HTTP inside the cluster plus a NetworkPolicy that admits only the gateway namespace. The body cap and server timeouts (read-header 5 s, read 10 s, write 10 s, idle 60 s) bound the load; SIGTERM shuts down gracefully.

## Rules cited in code comments

| ID | Rule |
|---|---|
| G13 | Fixed server timeouts (above). |
| G31 | Data after the JSON object gives 400. |
| G60 | `null` for an optional field is handled like any non-string: ignored, warning, WARN log with `*_type=null`. Only an empty `session_id` string counts as absent. |
| G61 | The recovery middleware logs the panic's Go type and the stack, never the value. |
| G62 | `PROBLEM_BASE_URL` must be absolute (a missing trailing `/` is added); `LOG_LEVEL` accepts slog levels; a lower-case code in `DATABASE_SANITY_CHECKS` is an error. |
| G63 | A body read failure other than the size limit is a 400 `malformed-request`. |
| G64 | HEAD on `/v1/ip-checks` is 405 with `Allow: POST`. |
| G65 | The reloader logs the rejection reason in `check` (`open`, `type`, `not_newer`, `sanity`) and does not reopen a rejected file until its modification time changes. |
| G66 | `run` logs its own startup and shutdown error once and returns it; `main` only exits with status 1. |
| G69 | `/metrics` uses the standard library. |
| G75 | Unknown field names are never logged, only their count. |
| G76 | An invalid `allowed_countries` entry is quoted back in the 422 reason, even if it looks like an IP. |

## Known limitations (do not "fix" without asking)

- VPNs and proxies are not detected (out of scope). The IP sent by the gateway is trusted.
- `XK` (Kosovo) can never be allowed: the contract is ISO 3166-1 alpha-2.
- **E11** A database file damaged inside with intact metadata is accepted: `/readyz` is 200 and checks return 500, never a wrong "allowed". Possible fix: call `Reader.Verify()` when loading.
- **E12** `PROBLEM_BASE_URL` with a query or fragment gives odd problem `type` values.
- **E13** Invalid UTF-8 in a JSON string is replaced, not rejected.
- With duplicate JSON keys the last one wins. A file rewritten without a change in modification time is not noticed. 6to4, Teredo and NAT64 addresses are not unwrapped.
- Stale-data policy is open (BQ2). May end users see `detail` texts? (BQ3, today: no, they are for the gateway and support.)

## Open for the gateway team

What should the gateway do in the "error" state (let in or block)? What is the real gateway namespace for the NetworkPolicy? What is the real `PROBLEM_BASE_URL`?
