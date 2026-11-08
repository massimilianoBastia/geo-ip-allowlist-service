# Build stage: static binary, version injected at build time.
FROM golang:1.26 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath \
    -ldflags "-s -w -X main.version=${VERSION}" \
    -o /out/geoip-service ./cmd/geoip-service

# Runtime stage: no shell, non-root. The database is not in the image; the
# update sidecar writes it to a shared volume (see deploy/kubernetes).
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/geoip-service /geoip-service
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/geoip-service"]
