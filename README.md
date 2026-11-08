# geo-ip-allowlist-service

A small Go HTTP service that answers one question: **is this IP address inside one of the allowed countries?**

The API gateway calls it before a UI login with the client IP and the customer's list of allowed countries. It looks the IP up in a MaxMind GeoLite2 Country database held in memory and answers `200` with `allowed: true|false` and a machine-readable `reason`. A blocked IP is a successful check, not an error. An IP with no known country (private, reserved, not in the data) is blocked: the service never fails open. The IP is never logged and never echoed.

```
user -> API gateway -> geo-ip-allowlist-service        update sidecar -> shared volume -> service (read-only)
```

## API in one minute

`POST /v1/ip-checks`, `Content-Type: application/json`. The full contract (every field, text and example) is [`api/openapi.yaml`](api/openapi.yaml) and nothing else.

```json
{"ip_address": "81.2.69.160", "allowed_countries": ["GB", "DE"], "session_id": "optional", "requested_at": "2026-11-08T17:00:00Z"}
```

```json
{"allowed": true, "reason": "country_allowed", "detail": "The IP address is located in GB (United Kingdom), which is in allowed_countries.", "country": {"iso_code": "GB", "name": "United Kingdom"}}
```

- `reason` is one of `country_allowed`, `country_not_allowed`, `country_unknown`, `country_unsupported`. Switch on `reason`, never parse `detail`.
- Gateway states: `200 allowed:true` let in, `200 allowed:false` block, any 4xx/5xx is "error".
- Every 4xx/5xx the service writes is RFC 9457 `application/problem+json` (400, 405, 413, 415, 422, 500, 503). Unknown paths get the plain standard-library 404.
- Other endpoints: `GET`/`HEAD` `/livez`, `/readyz`, `/version`, `/metrics`.

## Workflow A: run the Go binary (enough to judge the service)

Needs Go 1.26, `git`, `curl`. No credentials: it uses the synthetic database in `testdata/`. Run from the repository root.

**A1. Clone.** `git clone https://github.com/massimilianoBastia/geo-ip-allowlist-service.git && cd geo-ip-allowlist-service`

**A2. Build.** `go build -o bin/geoip-service ./cmd/geoip-service` (no output, exit 0).

**A3. Test.** `gofmt -l . ; go vet ./... ; go test -race -cover ./...` Expect no output from the first two and ten `ok` lines. (`-race` needs a C compiler; without one use `go test -cover ./...`.)

**A4. Run.**
```
GEOIP_DATABASE_PATH=testdata/GeoIP2-Country-Test.mmdb LISTEN_ADDR=127.0.0.1:8080 ./bin/geoip-service > /tmp/geoip.log 2>&1 &
for _ in $(seq 50); do curl -sf http://127.0.0.1:8080/livez >/dev/null && break; sleep 0.2; done; echo up
```
Prints `up`. The log has `"msg":"database loaded"` and a WARN `sanity lookups are off` (expected).

**A5. Ready?** `curl -s -w '\nHTTP %{http_code}\n' http://127.0.0.1:8080/readyz` gives `HTTP 200` and `{"database_built_at":"2026-10-22T18:52:57Z","database_type":"GeoIP2-Country","status":"ready"}`.

**A6. Call the API.**
```
check() { curl -s -w '\nHTTP %{http_code}\n' -X POST -H 'Content-Type: application/json' -d "$1" http://127.0.0.1:8080/v1/ip-checks; }
check '{"ip_address":"81.2.69.160","allowed_countries":["GB","DE"]}'   # 200 country_allowed
check '{"ip_address":"81.2.69.160","allowed_countries":["DE"]}'        # 200 country_not_allowed
check '{"ip_address":"10.0.0.1","allowed_countries":["GB"]}'           # 200 country_unknown, "country":null
check '{"ip_address":"nope","allowed_countries":["USA"]}'              # 422 problem+json, two invalid_params
```
Other test addresses are in [testdata/README.md](testdata/README.md).

**A7. Stop.** `pkill -x geoip-service`. Then `curl -s http://127.0.0.1:8080/livez` prints nothing (exit 7) and the log ends with `"msg":"server stopped"`.

## Workflow B (optional): Docker and Kubernetes

Finish A7 first so port 8080 is free. Requirements, the MaxMind account, Compose, `kind`, the Secret and every setting: [docs/deployment.md](docs/deployment.md).

```
docker build -t geo-ip-allowlist-service:dev .
docker run -d --name geoip-try -p 127.0.0.1:8080:8080 -v "$PWD/testdata:/data:ro,z" -e GEOIP_DATABASE_PATH=/data/GeoIP2-Country-Test.mmdb geo-ip-allowlist-service:dev
sleep 2; curl -s -w '\nHTTP %{http_code}\n' http://127.0.0.1:8080/readyz   # same body as A5
docker rm -f geoip-try
```

Without credentials: `docker compose -f docker-compose.test.yml up --build`. The manifest `deploy/kubernetes/geoip-service.yaml` (Deployment with the update sidecar, Service, NetworkPolicy) needs a MaxMind licence key; the gateway namespace `api-gateway` in the NetworkPolicy is a placeholder.

## Keeping the data fresh

A sidecar (`geoipupdate`, every 24 hours) writes `GeoLite2-Country.mmdb` to a shared volume. The service polls the file, and swaps it in atomically only if it opens, is newer and passes the sanity lookups; otherwise it keeps the old data and logs an ERROR. It starts without a database (`503` until the first file). The service never downloads the file and never sees the licence key; the real `.mmdb` is never committed.

## What is built, what is not

- **Built and tested:** the check endpoint, probes, `/metrics`, reload, Dockerfile, Compose, Kubernetes manifest.
- **Not built:** gRPC (designed in [docs/design-notes.md](docs/design-notes.md), cut for time) and VPN/proxy detection.
- **Run by the author, not covered by automated tests:** Docker and `kind` with a real MaxMind account (done with Podman; the unit tests never touch the real file or the network). Not tested: the daily update after the first download, clusters other than `kind`.
- **Known limitations:** a database damaged inside with intact metadata is accepted (E11); `XK` can never be allowed; stale-data policy is an open business question (BQ2). Full list: [docs/design-notes.md](docs/design-notes.md).

## Documents and AI use

| Topic | File |
|---|---|
| Contract | [api/openapi.yaml](api/openapi.yaml) |
| Reasons, limits, open questions | [docs/design-notes.md](docs/design-notes.md) |
| Run, Docker, Kubernetes, secrets | [docs/deployment.md](docs/deployment.md) |
| AI usage, time, test changes, session logs | [docs/ai-usage.md](docs/ai-usage.md) |

Claude Code wrote most of the code under the developer's direction; the process, its gaps and the time spent (about 4h30, above the 4h suggested) are stated in `docs/ai-usage.md`. Code is MIT ([LICENSE](LICENSE)); `testdata/GeoIP2-Country-Test.mmdb` is MaxMind's synthetic test data (Apache-2.0 / MIT).
