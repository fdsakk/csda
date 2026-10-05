#!/usr/bin/env python3
"""Write a manifest (name, size, SHA-256) of the reference demos in cs-demos/.

The demos come from https://gitlab.com/akiver/cs-demos (see download-demos.sh).
The hashes make a validation run reproducible: a result is only comparable to
another one if both used byte-identical demos.

  validation/manifest.py > validation/manifest.csv
"""
import csv, hashlib, os, sys

root = os.path.join(os.path.dirname(__file__), "..", "cs-demos")
writer = csv.writer(sys.stdout)
writer.writerow(["game", "file", "size_bytes", "sha256"])
for game in sorted(os.listdir(root)):
    folder = os.path.join(root, game)
    if not os.path.isdir(folder):
        continue
    for name in sorted(os.listdir(folder)):
        if not name.endswith(".dem"):
            continue
        digest = hashlib.sha256()
        with open(os.path.join(folder, name), "rb") as handle:
            for chunk in iter(lambda: handle.read(1 << 20), b""):
                digest.update(chunk)
        writer.writerow([game, name, os.path.getsize(os.path.join(folder, name)), digest.hexdigest()])
