# Watch verification cache measurement

On a frozen local corpus, reusing successful verification results reduced median
scan time from **25.30 seconds to 0.31 seconds**. A fresh process loading the disk
cache took **0.35 seconds**. All measured scans classified the same **312 sessions
as current and three as stale/unsupported**. The first cold scan still took 25.12
seconds; the cache does not remove initial content verification.

## Method and results

Measured on Linux amd64 with Go 1.27.1 on 2026-10-07. The corpus was copied from
one local CLI store: 300 SQLite sources, 16 legacy protobuf files (one shadowed by
SQLite), and 315 sidecars. Source and sidecar files occupied 1,071,977,791 bytes.
SQLite sources were copied through the read-only backup API to include committed
WAL contents. No private transcripts are included in this repository.

The opt-in `TestVerificationScanMeasurement` enumerates sessions and runs one
verification pass. Baseline uses the unchanged pre-cache `isStale` path. Warm runs
reuse the same frozen files. Restart runs execute in separate OS processes and
include reading/parsing the persisted cache in the timed region. Memory runs
prime the map before timing. CPU time is process user plus system CPU time, not
wall time multiplied by the process's lifetime CPU percentage.

| Mode | Samples | Median wall seconds | Median CPU seconds | SQLite checks / reused |
| --- | ---: | ---: | ---: | ---: |
| Before: full verification | 3 | 25.302 | 44.196 | 300 / 0 |
| After: cold cache | 1 | 25.118 | 43.420 | 300 / 0 |
| After: warm memory cache | 3 | 0.310 | 0.518 | 3 / 297 |
| After: restart with disk cache | 3 | 0.354 | 0.577 | 3 / 297 |

Warm memory scans used **98.8% less wall time and 98.8% less CPU time**. Restart
scans used **98.6% less wall time and 98.7% less CPU time**. The disk cache was
183,887 bytes for 297 entries. These are measurements of this corpus, not a
capacity claim or an estimate of model/API cost.

## Limits

This measures discovery and verification, not a complete live watch tick. It does
not include daemon discovery, trajectory fetching, parent-link resolution after
writes, or the existing refusal memo's checks. Unsupported sources still require
work; a corpus dominated by them would benefit less. Active changed sessions
still require full checks. Expired entries are rechecked four to five minutes
after their last full check, so these warm measurements are not a long-run CPU
average. The active user's watcher was not restarted for this experiment.

The operating-system file cache was warm, and the rest of the machine was not
quiesced. Repeated baseline timings were 25.785, 25.146 and 25.302 seconds;
restart timings including disk-cache loading were 0.354, 0.351 and 0.354 seconds.

## Reproduce

Use a private frozen copy of a conversation root with existing sidecars. Back up
SQLite through its backup API rather than copying an active `.db` without its
WAL. Preserve source/sidecar mtimes for legacy freshness checks. Keep the corpus
unchanged between modes and run restart measurements before entries expire.

```sh
export AGY_READER_BENCH_ROOT=/path/to/private/frozen-root
bench_dir=$(mktemp -d)
export AGY_READER_BENCH_CACHE="$bench_dir/cache"
go test -c -o "$bench_dir/measure.test" .
for mode in baseline baseline baseline cold restart restart restart memory memory memory; do
  AGY_READER_BENCH_MODE="$mode" "$bench_dir/measure.test" \
    -test.run='^TestVerificationScanMeasurement$' -test.v
 done
```

The test skips unless the corpus variable is set. It never contacts a daemon or
writes transcripts. JSON output records both classifications and timing. Use a
new cache directory for each cold-cache experiment; `cold` names the scenario,
not an instruction to delete an existing cache.
