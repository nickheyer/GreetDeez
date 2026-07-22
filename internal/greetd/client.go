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

// sane cap greetd replies are tiny
const maxMsgLen = 1 << 20

// talks greetd unix socket u32 le length then json
type Client struct {
	conn    net.Conn
	timeout time.Duration
}

// requests we send
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

// responses we get back
type Response struct {
	Type            string  `json:"type"`
	AuthMessageType *string `json:"auth_message_type"`
	AuthMessage     *string `json:"auth_message"`
	ErrorType       *string `json:"error_type"`
	Description     *string `json:"description"`
}

// timeout <= 0 falls back to default
func NewClient(timeout time.Duration) (*Client, error) {
	sock := os.Getenv("GREETD_SOCK")
	if sock == "" {
		return nil, fmt.Errorf("GREETD_SOCK not set — are you running under greetd?")
	}

	conn, err := net.Dial("unix", sock)
	if err != nil {
		return nil, fmt.Errorf("connect to greetd: %w", err)
	}

	if timeout <= 0 {
		timeout = defaultTimeout
	}
	return &Client{conn: conn, timeout: timeout}, nil
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
	if length > maxMsgLen {
		return nil, fmt.Errorf("message too large: %d bytes", length)
	}

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

// send then recv under deadline
func (c *Client) roundTrip(ctx context.Context, msg any) (*Response, error) {
	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(c.timeout)
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

// starts auth for username
func (c *Client) CreateSession(ctx context.Context, username string) (*Response, error) {
	return c.roundTrip(ctx, createSessionRequest{
		Type:     "create_session",
		Username: username,
	})
}

// answers current pam prompt nil acks info
func (c *Client) PostAuthResponse(ctx context.Context, response *string) (*Response, error) {
	return c.roundTrip(ctx, postAuthResponse{
		Type:     "post_auth_message_response",
		Response: response,
	})
}

// launches session with cmd and env
func (c *Client) StartSession(ctx context.Context, cmd []string, env []string) (*Response, error) {
	return c.roundTrip(ctx, startSessionRequest{
		Type: "start_session",
		Cmd:  cmd,
		Env:  env,
	})
}

// aborts in progress auth
func (c *Client) CancelSession(ctx context.Context) (*Response, error) {
	return c.roundTrip(ctx, cancelSessionRequest{
		Type: "cancel_session",
	})
}

// what a flattened Authenticate saw along the way
type AuthResult struct {
	Messages []string
}

// single password flow answers every secret with password
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
