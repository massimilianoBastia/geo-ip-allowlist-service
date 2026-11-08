# Deployment

## Requirements

What you need for each way of running the service. Install only the rows you use. Every tool has a command that shows it works.

| To do this | You need | Check it works |
|---|---|---|
| Build, test, run on a laptop | Go 1.26 or newer, `git`, `curl` | `go version` |
| Build and run the image | Docker or Podman | `docker version` or `podman version` |
| Run with Compose | Docker Compose v2 (or `podman compose`) | `docker compose version` |
| Apply the Kubernetes manifest | `kubectl` and a cluster | `kubectl version --client` |
| A local test cluster | `kind` (or `minikube`), plus Docker or Podman | `kind version` |
| The real GeoLite2 database | A free MaxMind account and licence key | see below |

`-race` in `go test -race` needs a C compiler (`gcc`). Without one, use `go test ./...`.

### Install the tools

Use your system's package manager or the official pages. The names differ by system, so the sources are:

- Go: https://go.dev/doc/install
- Docker: https://docs.docker.com/engine/install/ (Compose v2 is included in Docker Desktop and in the `docker-compose-plugin` package). Podman: https://podman.io/docs/installation
- kubectl: https://kubernetes.io/docs/tasks/tools/
- kind: https://kind.sigs.k8s.io/docs/user/quick-start/ . With Go installed it works on any system:

```
go install sigs.k8s.io/kind@v0.33.0
export PATH="$PATH:$(go env GOPATH)/bin"
kind version
```

(`v0.33.0` was the newest stable tag in `go list -m -versions sigs.k8s.io/kind` on 2026-11-08; use a newer one if you like.)

If you only have Podman, either install `podman-docker` (it provides a `docker` command) or run kind with `KIND_EXPERIMENTAL_PROVIDER=podman`. Rootless Podman also needs cgroup v2 delegation for kind; see the kind documentation on rootless mode. A cluster did start with rootless Podman and kind v0.33.0 on Fedora (cgroup v2, delegated controllers `cpu io memory pids`).

### Create a local cluster

```
kind create cluster --name geoip
kubectl cluster-info --context kind-geoip
```

The second command prints the address of the control plane. A kind node cannot see images from your machine, so after building the image you load it. With Docker:

```
docker build -t geo-ip-allowlist-service:0.1.0 .
kind load docker-image geo-ip-allowlist-service:0.1.0 --name geoip
```

With Podman (`KIND_EXPERIMENTAL_PROVIDER=podman`), `kind load docker-image` fails with `image ... not present locally`, because Podman names the image `localhost/geo-ip-allowlist-service:0.1.0` and the cluster looks for `docker.io/library/geo-ip-allowlist-service:0.1.0`. Tag it with the full name and load an archive:

```
podman build -t geo-ip-allowlist-service:0.1.0 .
podman tag localhost/geo-ip-allowlist-service:0.1.0 docker.io/library/geo-ip-allowlist-service:0.1.0
podman save -o /tmp/geoip-image.tar docker.io/library/geo-ip-allowlist-service:0.1.0
KIND_EXPERIMENTAL_PROVIDER=podman kind load image-archive /tmp/geoip-image.tar --name geoip
```

The default kind network may not enforce a `NetworkPolicy`. A test there proves the manifest applies, not that the policy blocks traffic.

Remove the cluster when done:

```
kind delete cluster --name geoip
```

### MaxMind account (only for the real database)

The test database needs no account. The real one does, and only the update container uses the credentials.

1. Create a free account: https://www.maxmind.com/en/geolite2/signup
2. In your account, under "Manage license keys", generate a key. Note your account ID.
3. Keep both outside the repository and never paste them into a chat, an issue or a log. How they reach the update container is described in the credentials section of this document.

## Settings

The service is configured only through environment variables. An unset or empty variable gets its default. An invalid value stops the service at start with one ERROR line that names the variable.

| Variable | Default | Meaning |
|---|---|---|
| `LISTEN_ADDR` | `:8080` | Address to listen on. Use `127.0.0.1:8080` on a laptop to stay off the network. |
| `GEOIP_DATABASE_PATH` | `/usr/share/GeoIP/GeoLite2-Country.mmdb` | Path of the `.mmdb` file. The service only reads it. |
| `DATABASE_POLL_INTERVAL` | `60s` | How often the file's modification time is checked. Go duration, must be positive. |
| `DATABASE_SANITY_CHECKS` | empty | Known pairs `ip=CC,ip=CC` checked before every swap, for example `81.2.69.160=GB,2001:218::1=JP`. Empty never stops the service: it logs one WARN `sanity lookups are off`. A malformed value, a trailing comma or a lower-case code is a start error. Use pairs that are true for the **real** file; those two are true for the test database only. |
| `MAX_REQUEST_BODY_BYTES` | `16384` | Largest accepted request body. Positive integer. |
| `PROBLEM_BASE_URL` | `https://geo-service.example/problems/` | Prefix of the `type` in error documents. Must be an absolute URL; a missing trailing `/` is added. The default is a placeholder: set it to a URL you control. |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn` or `error` (any case). |
| `SHUTDOWN_TIMEOUT` | `10s` | Time given to requests in flight after SIGTERM or SIGINT. Go duration, must be positive. |

The fixed server timeouts are in [api.md](api.md#limits). Reasons for each default: [api-design-notes.md](api-design-notes.md), H1.

## Run on a laptop

```
go build -ldflags "-X main.version=0.1.0" -o bin/geoip-service ./cmd/geoip-service
GEOIP_DATABASE_PATH=testdata/GeoIP2-Country-Test.mmdb LISTEN_ADDR=127.0.0.1:8080 ./bin/geoip-service
```

`-X main.version=...` sets what `/version` returns (default `dev`). With the real database, set `GEOIP_DATABASE_PATH` to your own `GeoLite2-Country.mmdb`; the file is not in the repository. The step-by-step check is Workflow A in the [README](../README.md#workflow-a-run-the-go-binary).

## Run with Docker

The image (`Dockerfile`) is a static binary on `gcr.io/distroless/static-debian12:nonroot`, user `nonroot` (uid 65532), no shell, and **no database**: mount one.

```
docker build --build-arg VERSION=0.1.0 -t geo-ip-allowlist-service:dev .
docker run -d --name geoip-try -p 127.0.0.1:8080:8080 \
  --read-only --cap-drop ALL --security-opt no-new-privileges \
  -v "$PWD/testdata:/data:ro,z" -e GEOIP_DATABASE_PATH=/data/GeoIP2-Country-Test.mmdb \
  geo-ip-allowlist-service:dev
curl -s -w '\nHTTP %{http_code}\n' http://127.0.0.1:8080/readyz
docker rm -f geoip-try
```

Expect `HTTP 200` with `"status":"ready"`. The `z` in the volume option lets SELinux hosts read the folder and is ignored elsewhere. The `--read-only`, `--cap-drop` and `no-new-privileges` flags are the same hardening the Compose files and the manifest use; the service needs none of what they remove.

## Run with Docker Compose

There are two files. Both publish the port on `127.0.0.1:8080` only.

### Without credentials: `docker-compose.test.yml`

Uses the synthetic test database; no MaxMind account, no secrets.

```
docker compose -f docker-compose.test.yml up --build
```

In another terminal: `curl -s -w '\nHTTP %{http_code}\n' http://127.0.0.1:8080/readyz` gives `HTTP 200`. Stop with `docker compose -f docker-compose.test.yml down`.

### With credentials: `docker-compose.yml`

Runs the service and the MaxMind update container (`geoipupdate`) that downloads `GeoLite2-Country.mmdb` into a shared volume. It needs your own MaxMind account ([requirements](#maxmind-account-only-for-the-real-database)).

```
cp secrets/geoipupdate_account_id.example secrets/geoipupdate_account_id
cp secrets/geoipupdate_license_key.example secrets/geoipupdate_license_key
# edit both new files: replace the placeholder text with your account id / licence key
docker compose up --build
```

`secrets/*` is ignored by git except the `*.example` files. At first the service answers `503` on `/readyz` while the download runs; it turns `200` when the file is loaded (the log shows `database loaded`). Set `DATABASE_SANITY_CHECKS` in the file (it is empty) with pairs that are true for the real database. The `geoipupdate` container has no read-only root filesystem in Compose, so it needs no `/tmp` volume there.

> Not tested: no Compose tool was available when this was written, and no MaxMind account was used. The service container settings of both files were run with `podman run`; the YAML files were parsed but not executed by Compose.

## Run on Kubernetes

`deploy/kubernetes/geoip-service.yaml` holds, in one namespace: a **Deployment** (2 replicas, each pod = the service + the `geoipupdate` sidecar sharing an `emptyDir` with the database), a **Service** (`ClusterIP`, port 80 to 8080) and a **NetworkPolicy**. The Secret is deliberately **not** in this file.

> What was tested: the manifest was applied to a `kind` v0.33.0 cluster (rootless Podman, `kubectl` v1.37.2) with the image built from this repository. Both containers started, the service loaded a database and answered through `kubectl port-forward`. What was **not** tested: the download from MaxMind (no licence key; the database file was copied in by hand, see [below](#test-on-kubernetes-without-a-licence-key)), the NetworkPolicy being enforced (not checked), and any cluster other than kind.

### What you must change first

| Where | Placeholder | What to do |
|---|---|---|
| Deployment, container `geoip-service` | `image: geo-ip-allowlist-service:0.1.0` | A name that exists only on your machine. Build and push the image to a registry the cluster can pull from, then use that name ([below](#build-tag-and-push-the-image)). With `kind`, load the image instead ([requirements](#create-a-local-cluster)). |
| NetworkPolicy | `kubernetes.io/metadata.name: api-gateway` | The name of the namespace the API gateway runs in. Only pods from that namespace may reach port 8080. With the wrong name nobody can reach the service, except through `kubectl port-forward`, which the policy does not block in most clusters. |
| Deployment, env `DATABASE_SANITY_CHECKS` | `""` | Pairs true for the real GeoLite2 file, or leave empty (one WARN). |
| (optional) env | `PROBLEM_BASE_URL` etc. | Add under `env:` any setting from the table above. |

### Create the Secret

Put the account id and the licence key in two local files outside the repository. They must hold only the value, and the file names must be `account_id` and `license_key`. Then:

```
kubectl create namespace geoip        # or use an existing namespace
kubectl -n geoip create secret generic maxmind-credentials \
  --from-file=account_id=./account_id \
  --from-file=license_key=./license_key
```

`deploy/kubernetes/secret.example.yaml` only shows the shape (name `maxmind-credentials`, keys `account_id`, `license_key`). **Do not apply it**: it holds placeholder text. The manifest mounts the Secret as read-only files at `/run/secrets` in the sidecar only, and the sidecar reads them through `GEOIPUPDATE_ACCOUNT_ID_FILE=/run/secrets/account_id` and `GEOIPUPDATE_LICENSE_KEY_FILE=/run/secrets/license_key`.

### Apply and check

```
kubectl -n geoip apply -f deploy/kubernetes/geoip-service.yaml
kubectl -n geoip rollout status deployment/geoip-service --timeout=180s
kubectl -n geoip get pods -l app=geoip-service
```

Ready looks like `deployment "geoip-service" successfully rolled out` and two pods with `READY 2/2` (both containers up; the readiness probe is `/readyz`, so a pod is ready only after the first database is loaded). If a pod stays `1/2`, read the logs:

```
kubectl -n geoip logs deploy/geoip-service -c geoip-service   # the service
kubectl -n geoip logs deploy/geoip-service -c geoipupdate     # the sidecar
```

The service logs `waiting for database file` (WARN) until the sidecar has written the file, then `database loaded`.

### Test with `kubectl port-forward`

```
kubectl -n geoip port-forward svc/geoip-service 8080:80
```

In another terminal, use the calls from the [README](../README.md#workflow-a-run-the-go-binary) (step A6) against `http://127.0.0.1:8080`. The IPs `81.2.69.160` and `2001:218::1` are in the **test** database only; with the real database use addresses you know the country of. Without any addresses to hand, `/readyz` returning `HTTP 200` with `"database_type":"GeoLite2-Country"` proves the real file is loaded.

### Test on Kubernetes without a licence key

This checks the manifest, the image, the probes and the reload, but not the download. Use a throwaway cluster ([requirements](#create-a-local-cluster)) with the image loaded. The order matters: the sidecar is pointed at an address that refuses connections **before** any pod can start, so it never contacts MaxMind with the placeholder values.

```
kubectl create namespace geoip
kubectl -n geoip apply -f deploy/kubernetes/geoip-service.yaml
kubectl -n geoip scale deployment/geoip-service --replicas=0
kubectl -n geoip set env deployment/geoip-service -c geoipupdate GEOIPUPDATE_HOST=http://127.0.0.1:9
printf 1 > /tmp/account_id; printf placeholder > /tmp/license_key
kubectl -n geoip create secret generic maxmind-credentials --from-file=account_id=/tmp/account_id --from-file=license_key=/tmp/license_key
kubectl -n geoip scale deployment/geoip-service --replicas=2
```

After a few seconds `kubectl -n geoip get pods` shows two pods `1/2 Running`: the service waits for a database (it logs `waiting for database file`). Copy the test database into each pod's shared volume:

```
for POD in $(kubectl -n geoip get pod -l app=geoip-service -o name); do
  kubectl -n geoip cp testdata/GeoIP2-Country-Test.mmdb ${POD#pod/}:/usr/share/GeoIP/GeoLite2-Country.mmdb -c geoipupdate
done
kubectl -n geoip rollout status deployment/geoip-service --timeout=150s
kubectl -n geoip port-forward svc/geoip-service 8080:80
```

The service polls every 60 s, so allow up to a minute. Expect `successfully rolled out` and two pods `2/2`. With the port-forward running, `curl -s http://127.0.0.1:8080/readyz` gives `HTTP 200` and the calls in the [README](../README.md#workflow-a-run-the-go-binary) (A5, A6) work. The sidecar restarts now and then, because its download is refused on purpose; that is expected. Delete everything with `kubectl delete namespace geoip`.

### Two details the sidecar needs

The `geoipupdate` container has a read-only root filesystem, but its image runs `mkdir -p /tmp/geoipupdate` at start and writes a health file there. Without a writable `/tmp` it exits at once with `mkdir: can't create directory '/tmp/geoipupdate': Read-only file system` (checked by running the image read-only as user 65532). The manifest therefore mounts an `emptyDir` (`update-tmp`) at `/tmp` in that container. The database folder is writable for user 65532 through `fsGroup: 65532`.

The pod sets `automountServiceAccountToken: false`. Without it, Kubernetes mounts the service-account token at `/var/run/secrets/kubernetes.io/serviceaccount`; `/var/run` is a link to `/run`, so the mount point would lie inside the read-only Secret volume at `/run/secrets` and the sidecar fails to start with `error mounting ... read-only file system` (seen on kind). The service never calls the Kubernetes API, so it does not need the token.

If a download fails (wrong key, no network), `geoipupdate` exits with an error and Kubernetes restarts that container with a back-off. The service container is not affected and keeps answering with the data it has.

## Keeping the database fresh

1. **Update.** The sidecar runs `geoipupdate` every 24 hours (`GEOIPUPDATE_FREQUENCY=24`) for the edition `GeoLite2-Country` and writes the file into the shared volume. The service never downloads anything and never sees the licence key.
2. **Detect.** The service checks the file's modification time every `DATABASE_POLL_INTERVAL` (default 60 s), and once at start. A file rewritten without changing its modification time is not noticed until the next change.
3. **Verify before swapping.** The new file must open, be a Country database, have a build date newer than the loaded one, and (if `DATABASE_SANITY_CHECKS` is set) answer every pair as expected. Only then is it swapped in memory; requests in flight are never dropped.
4. **On failure keep going.** A bad, older or removed file is logged at ERROR with the check that failed (`open`, `type`, `not_newer`, `sanity`), and the old data keeps answering. A rejected file is not retried until its modification time changes.
5. **Start with no file.** The service starts anyway, answers `503` on checks and `/readyz`, `200` on `/livez`, and logs one WARN. This gives the sidecar time to download for the first time, without a restart loop.
6. **Look at the age.** `/readyz` and `/version` return `database_built_at`; `/metrics` has `database_build_timestamp_seconds`. Alert on it if you need to.
7. **Each pod has its own copy.** The volume is an `emptyDir`, so a restarted pod downloads the file again, and every pod downloads separately.

Open business question (BQ2): should very old data ever stop the service from answering, and what does the GeoLite2 licence require about update frequency and deleting old copies? The default is to keep answering. The licence terms have not been checked.

## How credentials are handled

- The MaxMind account id and licence key exist only as **read-only mounted files in the update container**: Docker Compose `secrets:` (at `/run/secrets/<name>`) or a Kubernetes `Secret` mounted with `readOnly: true`.
- The update container reads them through `GEOIPUPDATE_ACCOUNT_ID_FILE` and `GEOIPUPDATE_LICENSE_KEY_FILE`. Those variables hold **file paths**, not secrets; the secret is inside the file.
- The service container never gets the Secret, the files or any credential variable.
- They are never in an environment value, in the image (no `COPY`, `ADD`, `ARG` or `ENV` for credentials), in the repository or in a log. Only `secrets/*.example` and `deploy/kubernetes/secret.example.yaml` (placeholders) are committed.
- If an image build ever needs a secret, use `docker buildx build --secret id=...` with `RUN --mount=type=secret`. The current Dockerfile needs none.
- A Secret object is only base64 in the cluster's store. Turn on encryption at rest and limit who can read Secrets, if the cluster is shared.

## Build, tag and push the image

```
VERSION=0.1.0
IMAGE=registry.example.com/your-team/geo-ip-allowlist-service
docker build --build-arg VERSION=$VERSION -t $IMAGE:$VERSION .
docker push $IMAGE:$VERSION
```

`registry.example.com/your-team` is a placeholder: use your registry, and log in first (`docker login registry.example.com`). `VERSION` is what `/version` reports. Use a fixed tag, never `latest`. Then point the manifest at it, either by editing `image:` or after applying:

```
kubectl -n geoip set image deployment/geoip-service geoip-service=$IMAGE:$VERSION
```

(The first `geoip-service` is the Deployment, the second the container name.) With Podman, replace `docker` by `podman`. The build was run with Podman; the push was not run, because there is no registry here.
