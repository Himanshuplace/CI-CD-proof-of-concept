# ── Stage 1: build ─────────────────────────────────────────────────────────
# We use the full golang image (has gcc, git) only during build.
# It never ends up in the final image.
ARG GO_VERSION=1.22
FROM golang:${GO_VERSION}-alpine AS build

WORKDIR /src

# Copy dependency files first.
# Docker caches layers. If go.mod/go.sum don't change, this RUN layer is cached
# and `go mod download` doesn't re-run on every code change. This saves ~30s
# per build.
COPY go.mod ./

# go mod tidy generates go.sum if not committed yet.
# In CI you'd commit go.sum; locally this generates it on first build.
RUN apk add --no-cache ca-certificates git && go mod tidy

COPY . .

# Build flags:
#   -trimpath  removes local file paths from the binary (don't leak /root/src/...)
#   -s -w      strips debug symbols and DWARF → ~30% smaller binary
#   CGO_ENABLED=0  pure Go binary, no libc dependency → works in distroless
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /out/service \
    ./cmd/server

# ── Stage 2: final image ────────────────────────────────────────────────────
# distroless/static contains: the binary, ca-certificates, tzdata.
# It has NO shell, NO package manager, NO utilities.
# An attacker who gains code execution has nothing to work with.
# Size: ~3MB vs ~150MB for Alpine.
FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app
COPY --from=build /out/service /app/service

# nonroot user (UID 65532) — never run production containers as root.
# Kyverno policy in policies/ enforces this at the cluster level too.
USER nonroot:nonroot

EXPOSE 8080
EXPOSE 9090

ENTRYPOINT ["/app/service"]
