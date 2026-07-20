# syntax=docker/dockerfile:1

# 1. Build the web UI (Vite + React) -> static assets in /web/dist.
# Pin Bun so different machines never rewrite the lockfile format.
FROM oven/bun:1.3.14 AS web
WORKDIR /web
COPY web/package.json web/bun.lock ./
RUN bun install --frozen-lockfile
COPY web/ ./
RUN bun run build

# 2. Download the map geometry used for visibility checks. Keeping this in a
# separate stage makes the build self-contained without adding Python/Awpy to
# the final image.
FROM python:3.13-slim AS geometry
RUN pip install --no-cache-dir awpy==2.0.2 \
    && awpy get tris

# 3. Build the Go server (pure Go sqlite driver -> static binary, no CGO)
FROM golang:1.23 AS server
ARG VERSION=dev
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# The source context intentionally excludes web/dist. Copy the freshly built
# dashboard so go:embed can compile and the server binary stays self-contained.
COPY --from=web /web/dist /src/web/dist
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w -X github.com/fdsakk/csda/pkg/cli.Version=${VERSION}" -o /out/csda ./cmd/cli

# 4. Minimal runtime image
FROM gcr.io/distroless/static-debian12
WORKDIR /app
COPY --from=server /out/csda /app/csda
COPY --from=web /web/dist /app/web/dist
# Map geometry for geometric visibility checks. Without it the analysis would
# fail (it refuses to silently fall back to the inaccurate spotted flag).
COPY --from=geometry /root/.awpy/tris /app/tris
EXPOSE 8080
# Bind to all interfaces so the port is reachable from the Windows host.
ENTRYPOINT ["/app/csda", "web", "--addr=0.0.0.0:8080", "--assets=/app/web/dist", "--db=/data/player-stats.db", "--uploads=/data/uploads"]
