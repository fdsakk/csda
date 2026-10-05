# Frozen version for the thesis

Tag `thesis-v1.0` points to the commit that contains this file. The code state it describes is commit `fddafda` (branch `dev`) plus this document. After the tag only errata are allowed: fix the error, tag `thesis-v1.0.1`, and record the change here.

## What the thesis describes

| Item | Value |
|---|---|
| Analysis version (stored in the database) | 8 |
| Export format | `cs-demo-analyzer/player-stats`, version 2 |
| Go (declared in `go.mod`) | 1.23.0, build/vet/tests verified with `GOTOOLCHAIN=go1.23.0` |
| Go (used for development and benchmarks) | 1.26.2 |
| Bun | 1.3.14 (minimum 1.3.0); frontend from `web/bun.lock` with `--frozen-lockfile` |
| Parser | `demoinfocs-golang/v4`, `replace` to `v4.5.2-0.20260706221401-7ea2e93e47b5` |
| Map geometry | Awpy 2.0.2, 18 maps; per-file SHA-256 in `scripts/tris.sha256`; archive `tris.zip` SHA-256 `f46ee3b99e6ff1adce78a6b0fb27266824b48232bd599d796351d314afcf61b7` |
| Reference demos | `validation/manifest.csv` (33 demos with size and SHA-256; 18 CS2) |
| Scope | `docs/SCOPE.md` (local, not tracked) |

## Parameters of the analysis (not configurable at runtime unless noted)

| Parameter | Value | Where |
|---|---|---|
| Consecutive visible ticks to confirm exposure | 3 (`--visibility-confirmation-ticks`) | `player_stats_collector.go` |
| Occlusion grace before an encounter resets | max(3 ticks, half a second) | `player_stats_collector.go` |
| Timing window used in aggregates | 0-1000 ms | `player_stats_report.go` |
| Under-190-ms rate threshold | 190 ms | `player_stats_report.go` |
| Moving shot | horizontal speed > 80 units | `player_stats_collector.go` |
| Snap | angle reduction >= 15 deg, <= 100 ms, final error <= 2 deg | `player_stats_collector.go` |
| View frustum | half angles 53.14 deg horizontal, 36.87 deg vertical (16:9) | `pkg/vis/engine.go` |
| Smoke | sphere of radius 155 units | `player_stats_collector.go` |
| Demo quality filter | `assessDemoQuality` | `player_stats_quality.go` |
| Review score thresholds (editable in the UI, stored in the database) | defaults below | `DefaultSuspicionConfig` in `player_stats.go` |

Default review thresholds (`GET /api/thresholds`, `defaults`):

```json
{"flagMode":"score","minimumDemos":3,"minimumShots":100,"ttdMinimumSamples":20,"ttdCheaterMs":320,"ttdSuspiciousMs":360,"reactionCheaterMs":200,"reactionWatchMs":240,"awpTtdCheaterMs":180,"awpTtdWatchMs":240,"eliteKd":1.8,"eliteHeadHitRate":0.4,"eliteAccuracy":0.3,"eliteKdCheater":3,"accuracyCheater":0.45,"headHitMinimumEvents":30,"headHitWatchThreshold":0.5,"headHitCheaterThreshold":0.6,"scoreWatchThreshold":45,"scoreCheaterThreshold":77,"metricWatchEvidence":0.3,"metricCheaterEvidence":0.85,"timingWeight":1,"awpTimingWeight":0.82,"awpEvidenceExponent":2,"precisionWeight":1,"performanceWeight":0.75,"synergyWeight":0.35,"sampleConfidenceFloor":0.3,"sampleConfidenceK":100,"scoreCurveExponent":0.65}
```

The review score is a prioritization aid, not a cheating probability.

## Verified on this version

- Clean clone: `go build`, `go vet`, `make test-unit` (race detector) pass; `bun install --frozen-lockfile && bun run build` leaves `git status` clean; `scripts/fetch-tris.sh <tris.zip>` verifies the geometry; `csda stats ingest` and `stats report` run on a real demo.
- Go 1.23.0: build, vet and tests pass.
- 18 CS2 reference demos: `TestRealDemoStatsAreConsistentWithTheMatch`, `TestRealDemoAnalysisIsDeterministic`, `TestRealDemoAnalysisIsCancellable`, `TestRealDemoHitsAreAttributedToTheirShot` pass.
- Time and memory: `validation/README.md` and `results-v8-*.csv`.

```sh
CSDA_TEST_DEMO=$(ls cs-demos/cs2/*.dem | paste -sd,) go test ./pkg/api -run TestRealDemo -timeout 40m -v
```

## Not done (limits of the evidence)

1. **No hand-checked reference.** Rounds, kills and first-visible ticks were not compared with the game. The real-demo tests prove internal consistency (collector vs match, determinism, cancellation, shot attribution), not agreement with what happened in the game. TTD and reaction values are model estimates with tick resolution (15.625 ms at 64 Hz) and were not validated against annotated encounters.
2. **Shot attribution without a hit** still uses the target closest to the crosshair and can pick the wrong target when two are visible.
3. **Player presence per round is not tracked**: ADR, KAST and the demo weights use all rounds of the match.
4. **K/D with zero deaths** falls back to the number of kills in the UI and the score; the review score ignores timing values equal to 0.
5. **Awpy download is unavailable** (HTTP 403 from `awpycs.com` on 2026-10-05); the geometry archive in the author's `tris/` is the only verified copy. `docker build` and the release workflow depend on that download and were not run. CI on GitHub was not run.
6. **Database written by earlier analysis versions** (older than 8) has wrong first-bullet statistics and must be rebuilt from the original demos.
7. **Job state is in memory only**: a restart loses the queue; imported demos stay in the database.
8. Browser checks were done in headless Chromium on loopback; no Windows build, installer or Docker run.
