package daemon

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"
)

// ReachesDaemon checks the daemon the way client's RPCs reach it. A direct
// connection gets the plain dial of DaemonReachable. When the client's
// transport sends the daemon URL through a proxy, a direct dial proves nothing
// either way. So the probe is a real RPC sent through the client with its CSRF
// token, counting as reachable only when it decodes as that RPC's response.
func ReachesDaemon(ctx context.Context, client *Client) error {
	u, err := url.Parse(client.BaseURL)
	if err != nil || !DaemonProxied(client.HTTP, u) {
		return DaemonReachable(client.BaseURL)
	}
	if err := RPCResponds(ctx, client); err != nil {
		return fmt.Errorf("daemon %s not reached through its proxy: %w", client.BaseURL, err)
	}
	return nil
}

// (c *Client) ReachesDaemon is a convenience method for ReachesDaemon.
func (c *Client) ReachesDaemon(ctx context.Context) error {
	return ReachesDaemon(ctx, c)
}

// RPCResponds sends the Client.CheckResponds probe through client, with a
// short timeout and without following redirects.
func RPCResponds(ctx context.Context, client *Client) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	httpClient := http.Client{}
	if client.HTTP != nil {
		httpClient = *client.HTTP
	}
	httpClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	probe := *client
	probe.HTTP = &httpClient
	return probe.CheckResponds(ctx)
}

// ClientProxied reports whether client's RPCs go through a proxy.
func ClientProxied(client *Client) bool {
	u, err := url.Parse(client.BaseURL)
	return err == nil && DaemonProxied(client.HTTP, u)
}

// (c *Client) IsProxied reports whether client's RPCs go through a proxy.
func (c *Client) IsProxied() bool {
	return ClientProxied(c)
}

// DaemonProxied reports whether httpClient's transport would send a request
// for u through a proxy.
func DaemonProxied(httpClient *http.Client, u *url.URL) bool {
	rt := http.DefaultTransport
	if httpClient != nil && httpClient.Transport != nil {
		rt = httpClient.Transport
	}
	t, ok := rt.(*http.Transport)
	if !ok || t.Proxy == nil {
		return false
	}
	proxyURL, err := t.Proxy(&http.Request{URL: u})
	return err != nil || proxyURL != nil
}

// DaemonReachable dials the host:port of a daemon base URL to confirm something
// is listening.
func DaemonReachable(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return err
	}
	if u.Host == "" {
		return fmt.Errorf("no host in daemon URL %q", rawURL)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("daemon URL %q is not http or https", rawURL)
	}
	conn, err := net.DialTimeout("tcp", DaemonDialAddr(u), 150*time.Millisecond)
	if err != nil {
		return err
	}
	conn.Close()
	return nil
}

// DaemonDialAddr is the host:port to probe for a daemon URL. A URL without an
// explicit port uses its scheme's default.
func DaemonDialAddr(u *url.URL) string {
	if u.Port() != "" {
		return u.Host
	}
	port := "80"
	if u.Scheme == "https" {
		port = "443"
	}
	return net.JoinHostPort(u.Hostname(), port)
}
