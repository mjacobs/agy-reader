package discovery

import "os"

// Connections returns only candidates admitted by the existing root/log/socket
// ownership checks. It adds no cross-root or alternate credential authority.
func Connections(root string) []Connection {
	candidates := discoverCLIConnections(root)
	if len(candidates) == 0 {
		if conn, err := DiscoverConnection(root); err == nil {
			candidates = append(candidates, conn)
		}
	}
	if token := os.Getenv("ANTIGRAVITY_CSRF_TOKEN"); token != "" {
		for i := range candidates {
			candidates[i].Token = token
		}
	}
	return candidates
}
