# Validation

## Reference demos

`manifest.csv` lists the 33 reference demos of `download-demos.sh` (source: https://gitlab.com/akiver/cs-demos) with size and SHA-256. Regenerate with `validation/manifest.py > validation/manifest.csv`.

## Consistency tests on real demos

```sh
CSDA_TEST_DEMO=/path/a.dem,/path/b.dem go test ./pkg/api -run TestRealDemo -v
```

- `TestRealDemoStatsAreConsistentWithTheMatch`: shots, damage and kills counted by the collector equal those of the match (accepted rounds only); hits <= shots; reaction <= TTD; ticks ordered.
- `TestRealDemoAnalysisIsDeterministic`: two analyses give identical statistics.
- `TestRealDemoAnalysisIsCancellable`: cancelling the context stops the parser within seconds.

Run on the 18 CS2 reference demos (including MatchZy backup restores and a recording that starts after the knife round): all pass. Before the per-round collector, one of them (`matchzy_iskandear_vs_kirill_2024_train`) counted 2969 shots against 2569 in the match.

These checks prove internal consistency, not correctness against the game. Hand-checked rounds and encounters (K/D, round boundaries, first visible tick) are still missing; see section 8.3 of `AUDYT_PRACY_INZYNIERSKIEJ.md`.

## Time and memory

`bench.py` runs `csda stats ingest --force` on a fresh database and records wall time and per-run peak RSS (maximum resident set size of the csda process, not a Go heap profile).

Environment: AMD Ryzen 7 7800X3D (16 threads), 30 GB RAM, Linux 7.0, Go 1.26.2, commit `53a5c4c` (analysis version 8). Warm page cache. Each `bench.py` run is a new process, so the geometry is loaded cold.

### Batch of four demos, CLI

`ebot_monte_vs_og_roobet_cup_2023_anubis`, `esplay_nvBBvqNCfFHV_2025_train`, `matchzy_aurora_vs_3dmax_m3_anubis`, `renown_match_8_2025_mirage` (1186 MiB in total), 3 runs each, raw data in `results-v8-batch4.csv`:

| jobs | time (median, range) | peak RSS (median, range) |
|---|---|---|
| 1 | 42.5 s (42.3-42.5) | 744 MiB (744-744) |
| 2 | 26.0 s (25.9-26.1) | 880 MiB (875-884) |
| 4 | 22.6 s (22.5-22.7) | 1026 MiB (1024-1026) |

Going from 1 to 4 jobs is only 1.9x faster: the largest demo of the batch is analyzed by one worker and sets the lower bound.

### Largest demo

`matchzy_bleed_vs_parivision_2024_mirage` (839 MiB), `jobs=1`, 3 runs (`results-v8-single-839mib.csv`): 21.5 s, peak RSS 168 MiB (168-170). Memory depends on the content of the demo, not on its file size: a 155 MiB demo (`ebot_monte_vs_og_roobet_cup_2023_anubis`) peaks at 411 MiB.

### Web flow (upload, analysis, report)

The same four demos uploaded to a running `csda web` (4 demos analyzed in parallel), one run each, measured with `curl` and `/api/jobs`:

| step | result |
|---|---|
| idle server | 22 MiB RSS |
| upload of 1186 MiB (loopback) | 1.3-1.4 s |
| analysis, cold geometry (first job) | 23.0 s, 4 imported, server peak RSS 1024 MiB |
| analysis, warm geometry (second job, same process, demos deleted in between) | 18.0 s, 4 imported, server peak RSS 1035 MiB |
| `GET /api/report` (40 players) | 6 ms |

Loading the map geometry costs about 5 s per process; the cache is kept for the lifetime of the server. The upload folder is empty after the job.

### Earlier numbers

The first version of this file reported 161 MiB / 58 s for the same batch with `jobs=1`. These values could not be reproduced: the original code (`174767c`, before the changes of this phase) measures 747 MiB on the same batch today, against 737 MiB for the final version. The figures above replace the earlier ones. The difference between the two versions is 1.4% for the batch. For the single 155 MiB demo the peak grew by 10% (373 to 411 MiB) between `174767c` and `797cd7d`; the cause was not isolated (the geometry is now loaded before parsing, which is one candidate).

Not measured: a cold page cache, machines other than the one above.
