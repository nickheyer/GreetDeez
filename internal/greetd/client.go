package greetd

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"time"
)

const defaultTimeout = 30 * time.Second

// Client communicates with greetd over its unix socket IPC protocol.
// Protocol: each message is a u32 length prefix (little-endian) followed by JSON.
type Client struct {
	conn net.Conn
}

// Request types sent to greetd
type createSessionRequest struct {
	Type     string `json:"type"`
	Username string `json:"username"`
}

type postAuthResponse struct {
	Type     string  `json:"type"`
	Response *string `json:"response"`
}

type startSessionRequest struct {
	Type string   `json:"type"`
	Cmd  []string `json:"cmd"`
	Env  []string `json:"env"`
}

type cancelSessionRequest struct {
	Type string `json:"type"`
}

// Response types received from greetd
type Response struct {
	Type            string  `json:"type"`
	AuthMessageType *string `json:"auth_message_type"`
	AuthMessage     *string `json:"auth_message"`
	ErrorType       *string `json:"error_type"`
	Description     *string `json:"description"`
}

func NewClient() (*Client, error) {
	sock := os.Getenv("GREETD_SOCK")
	if sock == "" {
		return nil, fmt.Errorf("GREETD_SOCK not set — are you running under greetd?")
	}

	conn, err := net.Dial("unix", sock)
	if err != nil {
		return nil, fmt.Errorf("connect to greetd: %w", err)
	}

	return &Client{conn: conn}, nil
}

func (c *Client) Close() error {
	if c == nil || c.conn == nil {
		return nil
	}
	return c.conn.Close()
}

func (c *Client) send(msg any) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	lenBuf := make([]byte, 4)
	binary.LittleEndian.PutUint32(lenBuf, uint32(len(data)))

	if _, err := c.conn.Write(lenBuf); err != nil {
		return fmt.Errorf("write length: %w", err)
	}
	if _, err := c.conn.Write(data); err != nil {
		return fmt.Errorf("write payload: %w", err)
	}
	return nil
}

func (c *Client) recv() (*Response, error) {
	lenBuf := make([]byte, 4)
	if _, err := io.ReadFull(c.conn, lenBuf); err != nil {
		return nil, fmt.Errorf("read length: %w", err)
	}
	length := binary.LittleEndian.Uint32(lenBuf)

	data := make([]byte, length)
	if _, err := io.ReadFull(c.conn, data); err != nil {
		return nil, fmt.Errorf("read payload: %w", err)
	}

	var resp Response
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	return &resp, nil
}

// roundTrip sends a message and reads the response with a context deadline.
func (c *Client) roundTrip(ctx context.Context, msg any) (*Response, error) {
	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(defaultTimeout)
	}
	if err := c.conn.SetDeadline(deadline); err != nil {
		return nil, fmt.Errorf("set deadline: %w", err)
	}
	defer c.conn.SetDeadline(time.Time{}) //nolint:errcheck

	if err := c.send(msg); err != nil {
		return nil, err
	}
	return c.recv()
}

// CreateSession initiates authentication for the given username.
func (c *Client) CreateSession(ctx context.Context, username string) (*Response, error) {
	return c.roundTrip(ctx, createSessionRequest{
		Type:     "create_session",
		Username: username,
	})
}

// PostAuthResponse sends the auth response (e.g. password) to greetd.
func (c *Client) PostAuthResponse(ctx context.Context, response *string) (*Response, error) {
	return c.roundTrip(ctx, postAuthResponse{
		Type:     "post_auth_message_response",
		Response: response,
	})
}

// StartSession tells greetd to launch the user's session with the given command
// and additional environment variables (e.g. XDG_SESSION_TYPE=wayland).
func (c *Client) StartSession(ctx context.Context, cmd []string, env []string) (*Response, error) {
	return c.roundTrip(ctx, startSessionRequest{
		Type: "start_session",
		Cmd:  cmd,
		Env:  env,
	})
}

// CancelSession cancels an in-progress authentication.
func (c *Client) CancelSession(ctx context.Context) (*Response, error) {
	return c.roundTrip(ctx, cancelSessionRequest{
		Type: "cancel_session",
	})
}

// AuthResult contains the outcome of an Authenticate call,
// including any informational PAM messages collected during the flow.
type AuthResult struct {
	Messages []string // non-secret auth messages (info, errors, MOTD, etc.)
}

// Authenticate handles the full create_session -> post_auth_response flow,
// cancelling any stale session first. Returns collected PAM info messages on success.
func (c *Client) Authenticate(ctx context.Context, username, password string) (*AuthResult, error) {
	c.CancelSession(ctx)

	resp, err := c.CreateSession(ctx, username)
	if err != nil {
		return nil, fmt.Errorf("create_session: %w", err)
	}

	if resp.Type == "error" {
		return nil, fmt.Errorf("greetd: %s", deref(resp.Description))
	}

	var messages []string
	for resp.Type == "auth_message" {
		var reply *string
		if resp.AuthMessageType != nil && *resp.AuthMessageType == "secret" {
			reply = &password
		} else if resp.AuthMessage != nil && *resp.AuthMessage != "" {
			// Collect non-secret messages (info, visible prompts, MOTD, etc.)
			messages = append(messages, *resp.AuthMessage)
		}

		resp, err = c.PostAuthResponse(ctx, reply)
		if err != nil {
			return nil, fmt.Errorf("post_auth: %w", err)
		}

		if resp.Type == "error" {
			return nil, fmt.Errorf("auth failed: %s", deref(resp.Description))
		}
	}

	if resp.Type != "success" {
		return nil, fmt.Errorf("unexpected response: %s", resp.Type)
	}
	return &AuthResult{Messages: messages}, nil
}

func deref(s *string) string {
	if s == nil {
		return "(no message)"
	}
	return *s
}
