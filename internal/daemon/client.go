package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const rpcPathPrefix = "/exa.language_server_pb.LanguageServerService/"

// CSRFHeader is validated by daemons that enforce CSRF authentication.
const CSRFHeader = "x-codeium-csrf-token"

var ErrAuthentication = errors.New("daemon authentication failed")

// Client talks to an Antigravity daemon's Connect-RPC endpoint — the same
// language server whether spawned by the CLI (`agy`) or the IDE.
//
// The daemon speaks plain JSON over HTTP — no protobuf on the wire. It only
// listens while its host program is running, and binds a different ephemeral
// port each invocation (see README troubleshooting).
type Client struct {
	BaseURL string
	HTTP    *http.Client
	// CSRFToken, when non-empty, is sent as the x-codeium-csrf-token header
	// on every RPC. Leave empty for older daemons that do not require it.
	CSRFToken string
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
	return c.getTrajectory(ctx, GetCascadeTrajectoryRequest{CascadeID: cascadeID})
}

// GetFullRawTrajectory exposes the persisted representation for comparison;
// callers retain the ordinary response for the public sidecar contract.
func (c *Client) GetFullRawTrajectory(ctx context.Context, cascadeID string) (*Trajectory, error) {
	return c.getTrajectory(ctx, struct {
		CascadeID          string `json:"cascadeId"`
		Verbosity          string `json:"trajectoryVerbosity"`
		DisableRehydration bool   `json:"disableRehydration"`
	}{cascadeID, "CLIENT_TRAJECTORY_VERBOSITY_FULL", true})
}

func (c *Client) getTrajectory(ctx context.Context, request any) (*Trajectory, error) {
	// Decode the envelope only. The trajectory object stays raw until we have
	// retained an owned copy, otherwise typed unmarshalling would discard new
	// daemon fields before cache.Write ever sees them.
	var resp struct {
		Trajectory json.RawMessage `json:"trajectory"`
	}
	if err := c.call(ctx, "GetCascadeTrajectory", request, &resp); err != nil {
		return nil, err
	}
	if len(resp.Trajectory) == 0 || string(resp.Trajectory) == "null" {
		// Preserve the client's historical loose response handling: a few daemon
		// control paths (and lightweight test doubles) return an empty envelope.
		// It decodes to the same zero Trajectory the old typed envelope produced.
		resp.Trajectory = json.RawMessage(`{}`)
	}
	var traj Trajectory
	if err := json.Unmarshal(resp.Trajectory, &traj); err != nil {
		return nil, fmt.Errorf("unmarshal GetCascadeTrajectory trajectory: %w", err)
	}
	traj.RawJSON = append(json.RawMessage(nil), resp.Trajectory...)
	return &traj, nil
}

// FetchTrajectory is the typical two-step convenience: load then get.
func (c *Client) FetchTrajectory(ctx context.Context, cascadeID string) (*Trajectory, error) {
	return c.FetchTrajectoryWithFallback(ctx, cascadeID, "")
}

// FetchTrajectoryWithFallback loads the session using loadID (the filename UUID)
// and gets the decrypted trajectory. If GetCascadeTrajectory fails and fallbackID
// is non-empty and distinct from loadID (e.g. when an SQLite session file's
// internal cascade_id differs from its filename), it retries GetCascadeTrajectory
// using fallbackID.
func (c *Client) FetchTrajectoryWithFallback(ctx context.Context, loadID, fallbackID string) (*Trajectory, error) {
	if err := c.LoadTrajectory(ctx, loadID); err != nil {
		return nil, fmt.Errorf("LoadTrajectory: %w", err)
	}
	traj, err := c.GetCascadeTrajectory(ctx, loadID)
	if err == nil || fallbackID == "" || fallbackID == loadID {
		return traj, err
	}
	fallbackTraj, fallbackErr := c.GetCascadeTrajectory(ctx, fallbackID)
	if fallbackErr == nil {
		return fallbackTraj, nil
	}
	// Both causes are wrapped, not formatted: watch mode tests the result with
	// errors.Is(err, ErrAuthentication) to reach its credential-recovery path,
	// and a %v-formatted fallback error would hide an authentication failure
	// that happened on the second attempt.
	return nil, fmt.Errorf("%w (fallback %s also failed: %w)", err, fallbackID, fallbackErr)
}

// CheckAuthentication checks a read-only RPC without loading any trajectory.
func (c *Client) CheckAuthentication(ctx context.Context) error {
	return c.call(ctx, "GetAllCascadeTrajectories", struct{}{}, nil)
}

// CheckResponds makes the read-only RPC of CheckAuthentication, with the
// client's CSRF token like every RPC, and succeeds only when the answer
// decodes as that RPC's response message. Something between the client and
// the daemon that answers on its own (an error, or a login page served as 200)
// fails it, whatever its status code.
//
// The response message is recognised by its trajectorySummaries map: the
// answer must be the empty object (protobuf JSON omits an empty map, so a
// daemon with no sessions sends {}) or carry trajectorySummaries as a JSON
// object. Other fields are allowed beside it, so a daemon that adds fields to
// this message still passes; any other JSON object, such as a proxy's
// {"error":"upstream unavailable"}, fails.
func (c *Client) CheckResponds(ctx context.Context) error {
	var out map[string]json.RawMessage
	if err := c.call(ctx, "GetAllCascadeTrajectories", struct{}{}, &out); err != nil {
		return err
	}
	if out == nil {
		return errors.New("GetAllCascadeTrajectories answered null, not a response message")
	}
	if len(out) == 0 {
		return nil
	}
	var summaries map[string]json.RawMessage
	if json.Unmarshal(out["trajectorySummaries"], &summaries) != nil || summaries == nil {
		return errors.New("answer is a JSON object without a trajectorySummaries map, not a GetAllCascadeTrajectories response")
	}
	return nil
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
	if c.CSRFToken != "" {
		req.Header.Set(CSRFHeader, c.CSRFToken)
	}

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
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			// Don't echo an authentication response body: it could contain a
			// credential supplied by the caller.
			return fmt.Errorf("%w: HTTP %d on %s (check CSRF token discovery or ANTIGRAVITY_CSRF_TOKEN)", ErrAuthentication, resp.StatusCode, method)
		}
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
