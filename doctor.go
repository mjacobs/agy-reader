package main

import (
	"github.com/mjacobs/agy-reader/internal/discovery"
)

// sidecarCoverage reports how many conversations/ sessions have a fresh
// sidecar versus a missing/stale one, reusing the same staleness rule the
// watcher uses (isStale).
func sidecarCoverage(root string) (total, fresh, stale int, err error) {
	sessions, err := discovery.ListConversationSessions(root)
	if err != nil {
		return 0, 0, 0, err
	}
	for _, s := range sessions {
		total++
		if isStaleNow, _ := isStale(s); isStaleNow {
			stale++
		} else {
			fresh++
		}
	}
	return total, fresh, stale, nil
}
