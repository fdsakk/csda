# Player Stats Web UI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a local React dashboard that uploads demos, processes them into the persistent player-statistics database, and refreshes an admin-focused player table.

**Architecture:** A standard-library Go HTTP server streams multipart uploads to a managed folder and runs the existing statistics builder in a single background queue. A Vite React client polls job state and the report API, with the production build served by the Go server from a configurable directory.

**Tech Stack:** Go `net/http`, existing SQLite statistics API, React 19, TypeScript, Vite, CSS.

---

### Task 1: Expose reports and upload jobs

**Files:**
- Modify: `pkg/api/player_stats_report.go`
- Create: `pkg/web/server.go`
- Test: `pkg/web/server_test.go`

- [x] Expose `GetPlayerStatsReport(ctx, options)` as the read-only report API.
- [x] Implement `POST /api/uploads`, `GET /api/jobs`, `GET /api/report`, and `GET /api/health`.
- [x] Stream `.dem` uploads to unique files, reject other extensions, enqueue one database build per upload batch, and expose queued/running/completed/failed status.
- [x] Verify multipart rejection, successful enqueue, report JSON, and static fallback with `go test ./pkg/web`.

### Task 2: Add the web CLI command

**Files:**
- Modify: `pkg/cli/cli.go`
- Create: `pkg/cli/web.go`

- [x] Add `csda web --db player-stats.db --uploads uploads --assets web/dist --addr 127.0.0.1:8080 --source valve` while preserving legacy and `stats` commands.
- [x] Start the HTTP server with graceful signal shutdown and clear startup output.
- [x] Verify the command with a static Go build and `/api/health` request.

### Task 3: Build the React admin console

**Files:**
- Create: `web/package.json`, `web/vite.config.ts`, `web/src/App.tsx`, `web/src/styles.css`

- [x] Create typed API models and parallel initial report/job loading.
- [x] Add a drag-and-drop multi-demo uploader with upload progress, source selection, error handling, and automatic table refresh after job completion.
- [x] Add summary counters, search/status filters, sortable player statistics, rule details, recurring-player emphasis, and responsive layout.
- [x] Build with `npm run build`, then verify the output through the Go static server.

### Task 4: Documentation and end-to-end verification

**Files:**
- Modify: `README.md`

- [x] Document `npm install`, `npm run build`, `csda web`, persistent uploads, and the database/report behavior.
- [x] Run `go test ./pkg/api ./pkg/web ./pkg/cli`, `go vet ./cmd/... ./pkg/...`, `npm run build`, and a health/report smoke test.
