package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const rpcPathPrefix = "/exa.language_server_pb.LanguageServerService/"

// Client talks to the Antigravity-CLI daemon's Connect-RPC endpoint.
//
// The daemon speaks plain JSON over HTTP — no protobuf on the wire, no auth.
// It only listens while `agy` is running, and binds a different ephemeral
// port each invocation (see README troubleshooting).
type Client struct {
	BaseURL string
	HTTP    *http.Client
}

// NewClient returns a Client pointing at baseURL with a 30s timeout. The
// caller is responsible for resolving the URL — there is no default, because
// the agy daemon binds a different ephemeral port each session.
func NewClient(baseURL string) *Client {
	return &Client{
		BaseURL: baseURL,
		HTTP:    &http.Client{Timeout: 30 * time.Second},
	}
}

// LoadTrajectory tells the daemon to decrypt and load a session into memory.
// Must be called before GetCascadeTrajectory.
func (c *Client) LoadTrajectory(ctx context.Context, cascadeID string) error {
	var out json.RawMessage
	return c.call(ctx, "LoadTrajectory", LoadTrajectoryRequest{CascadeID: cascadeID}, &out)
}

// GetCascadeTrajectory fetches the decrypted trajectory.
func (c *Client) GetCascadeTrajectory(ctx context.Context, cascadeID string) (*Trajectory, error) {
	var resp GetCascadeTrajectoryResponse
	if err := c.call(ctx, "GetCascadeTrajectory", GetCascadeTrajectoryRequest{CascadeID: cascadeID}, &resp); err != nil {
		return nil, err
	}
	return &resp.Trajectory, nil
}

// FetchTrajectory is the typical two-step convenience: load then get.
func (c *Client) FetchTrajectory(ctx context.Context, cascadeID string) (*Trajectory, error) {
	if err := c.LoadTrajectory(ctx, cascadeID); err != nil {
		return nil, fmt.Errorf("LoadTrajectory: %w", err)
	}
	return c.GetCascadeTrajectory(ctx, cascadeID)
}

func (c *Client) call(ctx context.Context, method string, body, out any) error {
	endpoint, err := url.JoinPath(c.BaseURL, rpcPathPrefix+method)
	if err != nil {
		return fmt.Errorf("build url: %w", err)
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	httpClient := c.HTTP
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("call %s: %w (is `agy` running?)", method, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode >= 300 {
		msg := string(respBody)
		var errEnv struct {
			Message string `json:"message"`
		}
		if json.Unmarshal(respBody, &errEnv) == nil && errEnv.Message != "" {
			msg = errEnv.Message
		}
		return fmt.Errorf("daemon error %d on %s: %s", resp.StatusCode, method, msg)
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(respBody, out); err != nil {
		return fmt.Errorf("unmarshal %s response: %w", method, err)
	}
	return nil
}
