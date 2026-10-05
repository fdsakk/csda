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

`bench.py` runs `csda stats ingest --force` on a fresh database and records wall time and per-run peak RSS.

Environment: AMD Ryzen 7 7800X3D (16 threads), 30 GB RAM, Linux, Go 1.26.2, commit 11f9101 plus the changes of this work. Warm page cache, geometry loaded per process (cold).

Four demos (1186 MiB in total), 3 runs each (`results-batch4-jobs.csv`):

| jobs | time | peak RSS |
|---|---|---|
| 1 | 58-59 s | about 161 MiB |
| 2 | 31 s | about 243 MiB |
| 4 | 21-22 s | about 393 MiB |

A single 161 MiB demo takes about 7.5 s at 150 MiB peak RSS, the same as before the changes (hashing the whole file adds negligible time). Not measured: warm geometry, demos above 700 MiB, the full web flow.
