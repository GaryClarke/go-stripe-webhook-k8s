########################################
# Stage 1: Build the Go binary
########################################

FROM golang:1.26 AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# BuildKit provides these automatically when using --platform.
# Defaults keep normal local builds simple.
ARG TARGETOS=linux
ARG TARGETARCH=amd64

# CGO_ENABLED=0 creates a static binary suitable for distroless/static.
# -trimpath removes local filesystem paths from the binary.
# -ldflags="-s -w" strips debug info to reduce binary size.
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api

RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w" -o /out/goose github.com/pressly/goose/v3/cmd/goose


########################################
# Stage 2: Minimal runtime image
########################################

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /out/api /api

# Explicit numeric non-root user for Kubernetes/OpenShift clarity.
USER 65532:65532

EXPOSE 8080

ENTRYPOINT ["/api"]


########################################
# Stage 3: In-cluster Goose migrations (private RDS — not reachable from GHA)
########################################

FROM debian:bookworm-slim AS migrate

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder --chmod=755 /out/goose /usr/local/bin/goose
COPY migrations /migrations
COPY --chmod=755 scripts/goose-up.sh /goose-up.sh

ENTRYPOINT ["/goose-up.sh"]