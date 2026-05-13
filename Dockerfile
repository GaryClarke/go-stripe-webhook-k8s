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


########################################
# Stage 2: Minimal runtime image
########################################

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /out/api /api

# Explicit numeric non-root user for Kubernetes/OpenShift clarity.
USER 65532:65532

EXPOSE 8080

ENTRYPOINT ["/api"]