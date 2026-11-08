# Review of the finished service

A review of the repository on 2026-11-08, done with Claude Code (see [ai-usage.md](ai-usage.md)). It ran the build, `go vet`, the race-enabled suite, a 30-second fuzz run of the request parser, the compiled binary against real HTTP requests, the container image, and a parse of the YAML files. No country-allowlist bypass was found. A real MaxMind download, the daily schedule and a Kubernetes rollout were not exercised.

## Findings

| # | Finding | Severity | Status |
|---|---|---|---|
| 1 | A corrupt MMDB file whose metadata is intact is accepted: `/readyz` returns 200 and checks return 500. `Reader.Verify()` would catch it. | medium | Open, known limitation |
| 2 | Unknown field names could carry an address into the DEBUG log. | medium | Fixed: only the count is logged (G75). An invalid `allowed_countries` entry is still quoted in the 422 reason (G76). |
| 3 | `requested_at` accepted values that are not RFC 3339, such as `T1:00:00Z` and `+24:00`. | low | Fixed: strict check and `TestParseRequestedAtRFC3339Profile` |
| 4 | `PROBLEM_BASE_URL` with a query or fragment gives odd `type` values. | low | Open, known limitation |
| 5 | Invalid UTF-8 inside a JSON string is replaced, not rejected. | low | Open, known limitation |

## Other known limitations
- **Stale data.** MaxMind requires GeoLite data to be kept current and old copies deleted within 30 days of a new release. The sidecar refreshes every 24 hours, but the service keeps answering from the data it has if updates fail. See BQ2 in the README.
- **gRPC** is designed only, not implemented.

## Tests added by the review
- `internal/httpapi/audit_test.go`: error precedence with no database loaded, the exact body-size boundary for chunked requests, and last-key-wins for duplicate JSON keys.
- `internal/request/audit_test.go`: a fuzz test of the parser invariants. Run it with `go test ./internal/request -run '^$' -fuzz FuzzParseInvariants -fuzztime=30s`.
