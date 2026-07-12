# TTD and Reaction Time Correction Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Correct Time to Damage methodology and add a separately reported estimated Reaction Time.

**Architecture:** Three spotted ticks validate an exposure but both clocks retain the first spotted tick as their origin. Damage encounters and shot reactions remain separate SQLite records so reaction samples include misses, while reports filter both metrics to 0–1000 ms and aggregate per-demo medians weighted by player rounds.

**Tech Stack:** Go collector/SQLite/report API, React TypeScript dashboard.

---

### Task 1: Correct collector semantics

**Files:** `pkg/api/player_stats_collector.go`, `pkg/api/player_stats_test.go`

- [x] Rename public TTM terminology to TTD and compute TTD from `firstTick` rather than `confirmedTick`.
- [x] Persist first-spot-to-first-shot reaction samples independently from damage encounters.
- [x] Test that confirmation preserves the original start tick and that samples over 1000 ms are excluded from report calculations.

### Task 2: Persist and aggregate reactions

**Files:** `pkg/api/player_stats.go`, `pkg/api/player_stats_report.go`

- [x] Add the reactions table without breaking existing SQLite databases and bump `analysis_version` so existing demos reprocess.
- [x] Report pooled median/P10 plus the primary round-weighted per-demo median for TTD and Reaction Time.
- [x] Rename JSON, CSV, scoring configuration, evidence kinds, and labels from TTM to TTD.

### Task 3: Update dashboard and verify samples

**Files:** `web/src/api.ts`, `web/src/App.tsx`, `README.md`

- [x] Add TTD and Reaction columns, sample counts, sorting, and expanded-row definitions.
- [x] Rebuild Go/React, reprocess the three sample demos, and verify the corrected distributions and UI API contract.
