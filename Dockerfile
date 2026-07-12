# syntax=docker/dockerfile:1

# 1. Build the web UI (Vite + React) -> static assets in /web/dist
FROM oven/bun:1 AS web
WORKDIR /web
COPY web/package.json web/bun.lock ./
RUN bun install --frozen-lockfile
COPY web/ ./
RUN bun run build

# 2. Build the Go server (pure Go sqlite driver -> static binary, no CGO)
FROM golang:1.23 AS server
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/csda ./cmd/cli

# 3. Minimal runtime image
FROM gcr.io/distroless/static-debian12
WORKDIR /app
COPY --from=server /out/csda /app/csda
COPY --from=web /web/dist /app/web/dist
EXPOSE 8080
# Bind to all interfaces so the port is reachable from the Windows host.
ENTRYPOINT ["/app/csda", "web", "--addr=0.0.0.0:8080", "--assets=/app/web/dist", "--db=/data/player-stats.db", "--uploads=/data/uploads"]
