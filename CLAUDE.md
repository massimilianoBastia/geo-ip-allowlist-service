# geo-ip-allowlist-service

Go service that answers one question: is this IP address inside one of the
allowed countries? Used by the API gateway to restrict UI logins by country.
The design is in `docs/design/` and `docs/api-design-notes.md`, and the
contracts in `api/`. If code and design disagree, the design wins: stop and ask.

## The repo at a glance

**Status:** built and tested (HTTP only). gRPC is designed, not built. VPN and
proxy detection is out of scope. Go 1.26; the only third-party dependency is
`github.com/oschwald/maxminddb-golang/v2` (plus `golang.org/x/sys`, indirect).

**What it does:** `POST /v1/ip-checks` with `{ip_address, allowed_countries[],
session_id?, requested_at?}` returns HTTP 200 with `{allowed, reason, detail,
country, session_id?, warnings?}`. `reason` is one of `country_allowed`,
`country_not_allowed`, `country_unknown`, `country_unsupported`. A blocked or
unknown IP is still 200 (`allowed: false`). Any 4xx/5xx is an RFC 9457
`application/problem+json` error. No country found means blocked (fail closed).

**Other endpoints:** `GET`/`HEAD` `/livez`, `/readyz`, `/version`, `/metrics`
(standard library, no Prometheus client).

**Packages** (`cmd/` is wiring only; everything else is under `internal/`):

| Package | Job |
|---|---|
| `cmd/geoip-service` | `main`: reads settings, builds and starts the parts, graceful shutdown |
| `config` | Settings from environment variables, validated |
| `iso` | The 249 assigned ISO 3166-1 alpha-2 codes |
| `netclass` | Parse and classify an IP (private, loopback, reserved...), log guard type |
| `request` | Decode and validate the request body, ordered `invalid_params`, warnings |
| `decision` | The allow/deny decision from a country record and the allowed set |
| `geoipdb` | Opens the `.mmdb` file and looks up a country; small `Database` interface |
| `reload` | Polls the file's modification time, verifies and atomically swaps the database |
| `httpapi` | Handlers, request pipeline, problem documents, recovery, ops endpoints, metrics |
| `tests/scenarios` | Acceptance tests against the committed test database |

**Settings (env vars, defaults in `internal/config/config.go`):** `LISTEN_ADDR`
(`:8080`), `GEOIP_DATABASE_PATH`, `PROBLEM_BASE_URL`, `DATABASE_POLL_INTERVAL`
(60s), `SHUTDOWN_TIMEOUT` (10s), `MAX_REQUEST_BODY_BYTES` (16384),
`DATABASE_SANITY_CHECKS`, `LOG_LEVEL`.

**Behaviour to remember:** the service starts without a database (checks and
`/readyz` return 503 until one loads, `/livez` is 200); a bad or older file is
rejected and the old data kept; the IP is never logged or echoed; unknown fields
are ignored and only their count is logged; invalid optional fields give a
warning, not an error; `allowed_countries` entries are upper-cased and checked
against the ISO list (422 if not); an empty list is 422.

**Deployment files:** `Dockerfile`, `docker-compose.yml` and
`docker-compose.test.yml`, `deploy/kubernetes/` (Deployment, Service,
NetworkPolicy, update sidecar, example Secret), `secrets/*.example`.

**Where to read what (fast):**

| Need | File |
|---|---|
| Overview, workflows to run and verify | `README.md` |
| Exact request/response/errors/headers | `docs/api.md`, `api/openapi.yaml`, `docs/problems.md` |
| Run, Docker, Kubernetes, secrets | `docs/deployment.md` |
| Why each decision, corner cases, known limitations (E) | `docs/api-design-notes.md` |
| How each decision was settled (G items, question log) | `docs/decision-log.md` |
| Design, edge cases (EC-xx), step order (S-xxx) | `docs/design/plan.md` |
| AI usage and edited session logs | `docs/ai-usage.md`, `docs/ai-log/` |

**Known limitations (do not "fix" without asking):** a database file damaged
inside with intact metadata is accepted (E11); `PROBLEM_BASE_URL` with a query
or fragment gives odd problem types (E12); invalid UTF-8 in a JSON string is
replaced, not rejected (E13); the stale-data policy is an open business question
(BQ2); `XK` can never be allowed.

**Quick commands:**

```
go build -o bin/geoip-service ./cmd/geoip-service
GEOIP_DATABASE_PATH=testdata/GeoIP2-Country-Test.mmdb LISTEN_ADDR=127.0.0.1:8080 ./bin/geoip-service
```

The full verified steps (build, run, four sample calls, stop, Docker, kind) are
in `README.md`, Workflows A and B.

## Working rules

- Don't make things up. If something is unclear while coding, stop and ask me
  before going on. Discuss design and library choices with me first.
- Do one plan step at a time, in the order of `docs/design/plan.md`.
- Never change or delete an existing test to make it pass. If a test looks
  wrong, tell me.
- Don't add features or files that the plan doesn't ask for.
- If you test the component follow strictly the workflow defined in the README.md either by building the binary or by installing the local cluster

## Go style

- Follow Effective Go (https://go.dev/doc/effective_go), the language spec
  (https://go.dev/ref/spec) and the code layout guide (https://go.dev/doc/code).
  Keep one style across the whole codebase.
- Standard library first. No third-party package without my approval; say why
  it is needed and what the alternative is.
- Small functions with a clear name that each do one short thing. Don't repeat
  yourself.
- Parts are independent and well encapsulated: depend on small interfaces and
  inject dependencies (constructors take them), so each part is testable alone.
- Errors: return them, wrap with `fmt.Errorf("...: %w", err)`, handle each once.
  No panics for expected failures.
- Pass `context.Context` as the first argument where work can block or cancel.
- Logging uses the standard `log/slog`, structured, with no secrets or full
  customer data in log lines.
- Comment exported names and anything non-obvious; no noise comments.
- For implementing the API-server for code organizations and project structure follow this layout https://go.dev/doc/modules/layout#server-project 

## Tests

- Tests are committed after the skeleton and before the implementation.
- Table-driven tests, with fakes for injected dependencies. No network in any
  test, and never the real GeoLite2 file.
- The committed synthetic database `testdata/GeoIP2-Country-Test.mmdb` (MaxMind
  test data, see `testdata/README.md`) is allowed and expected in the lookup
  and acceptance tests. Do not fake the lookup where the acceptance table
  needs that file; use fakes only for pure logic tests.
- Every edge case in the plan has a test.
- Before the tests, write the empty skeleton they need, in a separate commit:
  the packages, types and function signatures, with no logic. Bodies return a
  "not implemented" error or panic; where a value must be returned, return a
  deliberately wrong one, so a stub can never pass a test by accident. The
  tests must compile and then fail on their assertions (red), never on a
  missing symbol. Run them and confirm each one fails before implementing.
- Use a small interface only for a dependency that crosses a boundary (the
  database lookup, the clock, the logger) so tests can inject fakes. Pure
  functions stay plain functions.
- Order for each step: skeleton, then failing tests, then the implementation,
  each as its own commit. The implementation commit does not change tests,
  unless the developer approves it and the reason is recorded in
  `docs/decision-log.md`.

## Before finishing any step

```
gofmt -l .
go vet ./...
go test -race -cover ./...
```

Report what passed and what didn't.

## Data and secrets

- `GeoLite2-*.mmdb` is never committed (MaxMind licence). The service never
  downloads it and never touches the licence key: a separate update container
  (a sidecar) downloads it into a shared volume, and the service only reads
  the file from there, checks it, and swaps it in (`docs/api-design-notes.md`,
  D11 and J7).
- The MaxMind account id and licence key are mounted as read-only files into
  the update container only: Docker Compose `secrets:` (at
  `/run/secrets/<name>`) and, in Kubernetes, a `Secret` used as files with
  `readOnly: true`. They are read through the update tool's `_FILE` variables
  (`GEOIPUPDATE_ACCOUNT_ID_FILE`, `GEOIPUPDATE_LICENSE_KEY_FILE`). These
  variables hold file paths, not secrets: the secret is inside the file.
- Never put them in environment values, in the image (no `COPY`, `ADD`, `ARG`
  or `ENV` for credentials), or in the repository. If a build ever needs a
  secret, use `docker buildx build --secret` with `RUN --mount=type=secret`.
  Only `secrets/*.example` placeholders are committed.
- No keys, tokens or personal paths in code, config or logs.

## Git

- One commit per finished step, with a short subject line. Commit tests before
  the implementation that satisfies them.
