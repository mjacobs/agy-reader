//go:build linux

package main

import (
	"encoding/json"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/mjacobs/agy-reader/internal/discovery"
)

// Opt-in read-only measurement on a frozen, private corpus. The baseline calls
// exactly the pre-cache isStale path. No daemon, transcript writes or fixtures
// from the corpus enter the test suite or repository.
func TestVerificationScanMeasurement(t *testing.T) {
	root := os.Getenv("AGY_READER_BENCH_ROOT")
	if root == "" {
		t.Skip("set AGY_READER_BENCH_ROOT to a frozen session root")
	}
	mode := os.Getenv("AGY_READER_BENCH_MODE")
	var c *verificationCache
	if mode == "memory" {
		c = openVerificationCache(root, os.Getenv("AGY_READER_BENCH_CACHE"), verificationBootID())
	}
	scan := func() (int, int) {
		sessions, err := discovery.ListConversationSessions(root)
		if err != nil {
			t.Fatal(err)
		}
		fresh, stale := 0, 0
		for _, s := range sessions {
			bad, _ := c.isStale(s)
			if bad {
				stale++
			} else {
				fresh++
			}
		}
		return fresh, stale
	}
	if mode == "memory" {
		scan()
		c.reused, c.checked = 0, 0
	}
	var before, after syscall.Rusage
	syscall.Getrusage(syscall.RUSAGE_SELF, &before)
	start := time.Now()
	if mode != "baseline" && mode != "memory" {
		c = openVerificationCache(root, os.Getenv("AGY_READER_BENCH_CACHE"), verificationBootID())
	}
	fresh, stale := scan()
	if c != nil {
		if err := c.save(); err != nil {
			t.Fatal(err)
		}
	}
	elapsed := time.Since(start)
	syscall.Getrusage(syscall.RUSAGE_SELF, &after)
	cpu := func(r syscall.Rusage) float64 {
		return float64(r.Utime.Sec+r.Stime.Sec) + float64(r.Utime.Usec+r.Stime.Usec)/1e6
	}
	out := map[string]any{"mode": mode, "wall_seconds": elapsed.Seconds(), "cpu_seconds": cpu(after) - cpu(before), "fresh": fresh, "stale": stale}
	if c != nil {
		out["reused"] = c.reused
		out["checked"] = c.checked
	}
	raw, _ := json.Marshal(out)
	t.Log(string(raw))
}
