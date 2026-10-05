#!/usr/bin/env python3
"""Measure wall time and peak RSS of `csda stats ingest`.

Every run uses a fresh database and --force, so each run really analyzes the
demos instead of skipping them. Peak RSS is the maximum resident set size of
the csda process, not a Go heap profile.

Example:
  validation/bench.py --csda ./csda --tris-dir tris --jobs 1,2,4 --repeat 3 \
      --out validation/results.csv  demo1.dem demo2.dem
"""
import argparse, csv, os, platform, re, resource, subprocess, sys, tempfile, time


def run_once(csda, demos, jobs, tris_dir, extra):
    with tempfile.TemporaryDirectory() as tmp:
        db = os.path.join(tmp, "stats.db")
        cmd = [csda, "stats", "ingest", f"--db={db}", f"--jobs={jobs}", "--force", f"--tris-dir={tris_dir}"]
        for demo in demos:
            cmd.append(f"--demo={demo}")
        cmd += extra
        out_path = os.path.join(tmp, "stdout.txt")
        start = time.monotonic()
        with open(out_path, "w") as out:
            proc = subprocess.Popen(cmd, stdout=out, stderr=subprocess.DEVNULL)
            # wait4 reports the resource usage of this child alone, so the peak
            # RSS is per run and not a running maximum over all runs.
            _, status, usage = os.wait4(proc.pid, 0)
        elapsed = time.monotonic() - start
        proc.returncode = os.waitstatus_to_exitcode(status)
        with open(out_path) as handle:
            match = re.search(r"Imported: (\d+), skipped: (\d+), failed: (\d+)", handle.read())
        imported, skipped, failed = (int(x) for x in match.groups()) if match else (-1, -1, -1)
        return elapsed, usage.ru_maxrss / 1024, imported, skipped, failed, proc.returncode


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--csda", default="./csda")
    parser.add_argument("--tris-dir", default="tris")
    parser.add_argument("--jobs", default="1")
    parser.add_argument("--repeat", type=int, default=3)
    parser.add_argument("--out", default="")
    parser.add_argument("demos", nargs="+")
    args, extra = parser.parse_known_args()

    sizes = sum(os.path.getsize(d) for d in args.demos) / 2**20
    rows = []
    for jobs in (int(j) for j in args.jobs.split(",")):
        for repeat in range(1, args.repeat + 1):
            elapsed, rss, imported, skipped, failed, code = run_once(args.csda, args.demos, jobs, args.tris_dir, extra)
            row = dict(jobs=jobs, repeat=repeat, demos=len(args.demos), demo_mib=round(sizes, 1), seconds=round(elapsed, 2),
                       peak_rss_mib=round(rss, 1), imported=imported, skipped=skipped, failed=failed, exit_code=code,
                       cpu=platform.processor() or platform.machine(), cores=os.cpu_count(), os=platform.platform())
            rows.append(row)
            print(row, flush=True)
    if args.out:
        with open(args.out, "w", newline="") as handle:
            writer = csv.DictWriter(handle, fieldnames=list(rows[0]))
            writer.writeheader()
            writer.writerows(rows)


if __name__ == "__main__":
    sys.exit(main())
