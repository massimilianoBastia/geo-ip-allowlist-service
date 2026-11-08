# geo-ip-allowlist-service

A small Go HTTP service that answers one question: **is this IP address inside one of the allowed countries?**

- The API gateway calls it before a UI login, with the client IP and the list of allowed countries.
- It looks the IP up in a MaxMind GeoLite2 Country database, held in memory.
- It answers `200` with `allowed: true` or `false` and a machine-readable `reason`. A blocked IP is a successful check, not an error.
- An IP with no known country (private, reserved, not in the database) is blocked: the service never fails open.
- The IP is never logged and never echoed in a response.

Where it sits: `user -> API gateway -> geo-ip-allowlist-service`. A separate update container downloads the database; the service only reads it ([docs/deployment.md](docs/deployment.md)).

## API at a glance

`POST /v1/ip-checks` with `Content-Type: application/json`.

| Request field | Required | Rule |
|---|---|---|
| `ip_address` | yes | IPv4 or IPv6 text address |
| `allowed_countries` | yes | non-empty array of ISO 3166-1 alpha-2 codes (`gb` is read as `GB`) |
| `session_id` | no | string, at most 128 characters; echoed back when valid |
| `requested_at` | no | RFC 3339 timestamp; only logged |

Unknown fields are ignored. A bad `session_id` or `requested_at` is ignored with a warning, not rejected.

| Response field | Meaning |
|---|---|
| `allowed` | `true` only when `reason` is `country_allowed` |
| `reason` | `country_allowed`, `country_not_allowed`, `country_unknown` (no country found), `country_unsupported` (country code is not ISO alpha-2, today `XK`) |
| `detail` | sentence for people; do not parse it |
| `country` | `{"iso_code","name"}`, or `null` for `country_unknown` |
| `session_id`, `warnings` | only when present |

Request:

```json
{
  "ip_address": "81.2.69.160",
  "allowed_countries": [
    "GB",
    "DE"
  ],
  "session_id": "3f1b5c9e-7a4d-4a64-9d2e-6d7a1f0c8b11",
  "requested_at": "2026-11-08T17:00:00Z"
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
  },
  "session_id": "3f1b5c9e-7a4d-4a64-9d2e-6d7a1f0c8b11"
}
```

A blocked IP is still `200`, with `allowed: false` and the cause in `reason` and `detail`. The same request with `"allowed_countries": ["DE"]` returns:

```json
{
  "allowed": false,
  "reason": "country_not_allowed",
  "detail": "The IP address is located in GB (United Kingdom), which is not in allowed_countries, so it is blocked.",
  "country": {
    "iso_code": "GB",
    "name": "United Kingdom"
  },
  "session_id": "3f1b5c9e-7a4d-4a64-9d2e-6d7a1f0c8b11"
}
```

With a private address (`"ip_address": "10.0.0.1"`) no country can be found, so `country` is `null`:

```json
{
  "allowed": false,
  "reason": "country_unknown",
  "detail": "The IP address is in a private range and has no associated country, so it is blocked.",
  "country": null
}
```

Every 4xx and 5xx is an [RFC 9457](https://www.rfc-editor.org/rfc/rfc9457) `application/problem+json` document. For the gateway, any 4xx or 5xx means "error". The `type` is `PROBLEM_BASE_URL` (default `https://geo-service.example/problems/`) plus the name below.

| Status | Problem type | When | Example that causes it |
|---|---|---|---|
| 400 | `malformed-request` | body is not valid JSON | body `{bad` |
| 405 | `method-not-allowed` | not `POST` on the check; not `GET`/`HEAD` on `/livez`, `/readyz`, `/version`, `/metrics` | `GET /v1/ip-checks` |
| 413 | `request-too-large` | body above `MAX_REQUEST_BODY_BYTES` (default 16384) | a 16385-byte body |
| 415 | `unsupported-media-type` | `Content-Type` missing or not `application/json` | `curl -d '{}'` with no header |
| 422 | `invalid-request` | JSON is not an object, a required field is missing or invalid, or the list is empty; all problems are listed | `{"ip_address":"nope","allowed_countries":["USA"]}` |
| 500 | `internal-error` | lookup error or panic; no internals in the body | cannot be triggered on purpose |
| 503 | `database-unavailable` | valid check, or `/readyz`, while no database is loaded | start with a missing database file |

All problem bodies have `type`, `title`, `status`, `detail`; only 422 adds `invalid_params`. The 422 example above answers:

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

The exact body of every problem is in [docs/api.md](docs/api.md). Other endpoints: `GET /livez`, `GET /readyz`, `GET /version`, `GET /metrics`.

## Quickstart

Needs Go 1.26 and `curl`. No MaxMind account is needed: it uses the synthetic test database in `testdata/`.

```
GEOIP_DATABASE_PATH=testdata/GeoIP2-Country-Test.mmdb LISTEN_ADDR=127.0.0.1:8080 go run ./cmd/geoip-service
```

In a second terminal:

```
curl -s -X POST -H 'Content-Type: application/json' \
  -d '{"ip_address":"81.2.69.160","allowed_countries":["GB"]}' http://127.0.0.1:8080/v1/ip-checks
```

You should see `"allowed":true,"reason":"country_allowed"`. In the test database `81.2.69.160` is in GB; the other test addresses are in [testdata/README.md](testdata/README.md).

## Install and deploy to test

Details, settings and credentials: [docs/deployment.md](docs/deployment.md). What to install for each option (Docker, `kubectl`, a local `kind` cluster): [requirements](docs/deployment.md#requirements).

- **Laptop (the Go binary):** `go build -o bin/geoip-service ./cmd/geoip-service`, then run it with the variables above. This is [Workflow A](#workflow-a-run-the-go-binary) and is all a reviewer needs.
- **Docker (optional):** `docker build -t geo-ip-allowlist-service:dev .`; the image holds no database, so mount one ([Workflow B](#workflow-b-optional-docker-and-kubernetes), step B1). With Compose and no credentials: `docker compose -f docker-compose.test.yml up --build`.
- **Kubernetes (optional):** `deploy/kubernetes/geoip-service.yaml` (Deployment with an update sidecar, Service, NetworkPolicy). It needs a MaxMind licence key for the sidecar; see the deployment document.

## Workflow: build, run and verify

For a reviewer or a coding agent. There are two workflows. Each step has a command, the expected result and how to tell it worked. Run the commands from the repository root. Neither needs credentials.

| | Workflow A | Workflow B |
|---|---|---|
| What | Run the Go binary itself | Run the container image and read the Kubernetes manifest |
| Status | **Start here; enough to judge the service** | **Optional** |
| Needs | Go 1.26, `git`, `curl` | Docker or Podman for B1; nothing extra for B2 (a cluster and `kubectl` only to deploy, see [requirements](docs/deployment.md#requirements)) |

## Workflow A: run the Go binary

**A1. Clone.**
```
git clone https://github.com/massimilianoBastia/geo-ip-allowlist-service.git && cd geo-ip-allowlist-service
```
Worked if `ls go.mod Dockerfile api/openapi.yaml testdata/GeoIP2-Country-Test.mmdb` lists all four files.

**A2. Build.**
```
go build -o bin/geoip-service ./cmd/geoip-service
```
No output; exit code 0; `bin/geoip-service` exists. (If your Go is older than 1.26, `go` downloads the toolchain named in `go.mod`, which needs network.)

**A3. Test.**
```
gofmt -l . ; go vet ./... ; go test -race -cover ./...
```
`gofmt -l .` and `go vet` print nothing. `go test` prints ten lines starting with `ok` and no `FAIL`. (`-race` needs a C compiler; without one, use `go test -cover ./...`.) No test uses the network or a real GeoLite2 file.

**A4. Run with the test database** (in the background; the second line waits until it is ready).
```
GEOIP_DATABASE_PATH=testdata/GeoIP2-Country-Test.mmdb LISTEN_ADDR=127.0.0.1:8080 ./bin/geoip-service > /tmp/geoip.log 2>&1 &
until curl -sf http://127.0.0.1:8080/livez >/dev/null; do sleep 0.2; done; echo up
```
Prints `up`. `/tmp/geoip.log` holds JSON lines, including `"msg":"database loaded"` and a WARN `sanity lookups are off` (expected: no sanity pairs are configured).

**A5. Check readiness.**
```
curl -s -w '\nHTTP %{http_code}\n' http://127.0.0.1:8080/readyz
```
Expect `HTTP 200` and `{"database_built_at":"2026-10-22T18:52:57Z","database_type":"GeoIP2-Country","status":"ready"}`.

**A6. Call the API.** Four calls; the expected status and key fields are in the comment.
```
# allowed -> HTTP 200, "allowed":true,"reason":"country_allowed"
curl -s -w '\nHTTP %{http_code}\n' -X POST -H 'Content-Type: application/json' -d '{"ip_address":"81.2.69.160","allowed_countries":["GB","DE"]}' http://127.0.0.1:8080/v1/ip-checks

# blocked -> HTTP 200, "allowed":false,"reason":"country_not_allowed"
curl -s -w '\nHTTP %{http_code}\n' -X POST -H 'Content-Type: application/json' -d '{"ip_address":"81.2.69.160","allowed_countries":["DE"]}' http://127.0.0.1:8080/v1/ip-checks

# private IP -> HTTP 200, "allowed":false,"reason":"country_unknown","country":null
curl -s -w '\nHTTP %{http_code}\n' -X POST -H 'Content-Type: application/json' -d '{"ip_address":"10.0.0.1","allowed_countries":["GB"]}' http://127.0.0.1:8080/v1/ip-checks

# error -> HTTP 422, application/problem+json, two entries in invalid_params
curl -s -w '\nHTTP %{http_code}\n' -X POST -H 'Content-Type: application/json' -d '{"ip_address":"nope","allowed_countries":["USA"]}' http://127.0.0.1:8080/v1/ip-checks
```

**A7. Stop the service.**
```
pkill -x geoip-service
```
Worked if `curl -s http://127.0.0.1:8080/livez` now prints nothing and exits with code 7 (connection refused). The log ends with `"msg":"server stopped"`.

## Workflow B (optional): Docker and Kubernetes

Do Workflow A first, and finish its step A7 so port 8080 is free.

**B1. Build and run the image** (use `podman` in place of `docker` if you prefer). The image holds no database, so the test file is mounted read-only.
```
docker build -t geo-ip-allowlist-service:dev .
docker run -d --name geoip-try -p 127.0.0.1:8080:8080 -v "$PWD/testdata:/data:ro,z" -e GEOIP_DATABASE_PATH=/data/GeoIP2-Country-Test.mmdb geo-ip-allowlist-service:dev
sleep 2; curl -s -w '\nHTTP %{http_code}\n' http://127.0.0.1:8080/readyz
docker rm -f geoip-try
```
The build ends with a "Successfully tagged" or "writing image" line. `/readyz` gives `HTTP 200`, the same body as step A5. (The `z` lets SELinux hosts read the folder and is ignored elsewhere.)

**B2. Read the manifest** (no cluster needed).
```
grep -n '^kind:' deploy/kubernetes/geoip-service.yaml
grep -nE 'image:|readOnlyRootFilesystem|secretName' deploy/kubernetes/geoip-service.yaml
```
The first lists the objects (Deployment, Service, NetworkPolicy). The second shows two `image:` lines (the service image, a placeholder name; the MaxMind update image), two `readOnlyRootFilesystem: true`, and the Secret name the sidecar reads. Details of each object: [docs/deployment.md](docs/deployment.md).

Deploying to a cluster needs `kubectl`, a cluster and a MaxMind licence key. It is described in [docs/deployment.md](docs/deployment.md), not here.

## Assumptions

Short reasons; the full list with sources is in [docs/api-design-notes.md](docs/api-design-notes.md). If the notes and another document disagree, the notes win.

- **A blocked IP is `200`, not an error.** The gateway has three states: allowed, not allowed, error.
- **No country means blocked** (`country_unknown`), and there is no fallback to `registered_country`: failing open would let anyone bypass the restriction.
- **`XK` (Kosovo) can never be allowed.** The contract is ISO 3166-1 alpha-2 and `XK` is not in it.
- **`POST` with a JSON body**, so the IP stays out of access-log URLs.
- **An invalid country code is a 422, not ignored**, because silently dropping it would block that country's users with no trace.
- **An IPv6 zone (`fe80::1%eth0`) is dropped, not rejected**; it names a local interface, not a country.
- **The service starts without a database** and answers `503` until the first file arrives, so the sidecar has time to download it.
- **A bad or older database file is rejected** and the old data kept, with an ERROR log.
- **Plain HTTP inside the cluster**, limited by a NetworkPolicy to the gateway namespace. The IP the gateway sends is trusted.
- **Very old data still answers.** Whether it should stop is an open business question (BQ2).

## What is built and what is only designed

- **Built and tested:** the check endpoint, `/livez`, `/readyz`, `/version`, `/metrics`, reload of the database file, and the Dockerfile (built and run with Podman).
- **Designed only:** gRPC (design in the notes, section F2), VPN and proxy detection (not planned; known limitation).
- **Written but not run:** the two Compose files (their service container settings were run with `podman run`).
- **Run on a local cluster:** the Kubernetes manifest was applied to a local `kind` cluster, and the service answered through `kubectl port-forward` ([docs/deployment.md](docs/deployment.md#run-on-kubernetes)).
- **Not tested:** the real GeoLite2 file (tests never use it) and the update sidecar's download.

## Use of AI

Claude Code was used under the developer's direction; the developer made the design decisions. See [docs/ai-usage.md](docs/ai-usage.md).

## Licence and data

- The code is under the MIT licence ([LICENSE](LICENSE)), as declared in `api/openapi.yaml`.
- `testdata/GeoIP2-Country-Test.mmdb` is MaxMind's synthetic test data, under Apache-2.0 / MIT ([testdata/README.md](testdata/README.md)).
- The real `GeoLite2-Country.mmdb` is under MaxMind's GeoLite2 licence. It is **never committed** and the service never downloads it. Only the update container does, with your own MaxMind account.
- MaxMind credentials are only ever read-only mounted files in the update container: never environment values, never in the image, never in the repository. Only `secrets/*.example` placeholders are committed.

## Where to find what

| Topic | Authority |
|---|---|
| Decisions and reasons | [docs/api-design-notes.md](docs/api-design-notes.md) |
| Machine-readable contract | [api/openapi.yaml](api/openapi.yaml) |
| Problem types | [docs/problems.md](docs/problems.md) |
| API detail | [docs/api.md](docs/api.md) |
| Running and deploying | [docs/deployment.md](docs/deployment.md) |
