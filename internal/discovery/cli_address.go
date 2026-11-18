package discovery

import (
	"net"
	"strconv"
	"strings"
)

// localAddressKey accepts only literal loopback endpoints (localhost retains
// its historical IPv4 meaning). Credentials never cross address families merely
// because two listeners have the same port number.
func localAddressKey(address string) string {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return ""
	}
	switch host {
	case "localhost", "127.0.0.1":
		host = "127.0.0.1"
	case "::1":
	default:
		return ""
	}
	n, err := strconv.ParseUint(port, 10, 16)
	if err != nil || n == 0 {
		return ""
	}
	return net.JoinHostPort(host, strconv.FormatUint(n, 10))
}

func sameLocalEndpoint(a, b string) bool {
	const prefix = "http://"
	if !strings.HasPrefix(a, prefix) || !strings.HasPrefix(b, prefix) {
		return false
	}
	address := localAddressKey(strings.TrimSuffix(strings.TrimPrefix(a, prefix), "/"))
	return address != "" && address == localAddressKey(strings.TrimPrefix(b, prefix))
}
