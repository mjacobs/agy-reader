//go:build !linux

package discovery

// Other platforms retain log discovery and the explicit CSRF override.
func discoverCLIEndpoint(root, wantedURL string) (string, string) {
	return "", ""
}
