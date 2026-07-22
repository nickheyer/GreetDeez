package rpc

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"google.golang.org/protobuf/proto"

	pb "github.com/nickheyer/greetdeez/gen/go/transport/v1"
)

const defaultCallTimeout = 60 * time.Second

// SocketClient speaks the greetdeez protocol over the unix socket transport.
// Reference client for out-of-process UIs; the metal theme runs on it.
type SocketClient struct {
	// one call in flight per connection
	mu      sync.Mutex
	conn    net.Conn
	timeout time.Duration
}

// timeout <= 0 falls back to default
func DialSocket(path string, timeout time.Duration) (*SocketClient, error) {
	conn, err := net.Dial("unix", path)
	if err != nil {
		return nil, fmt.Errorf("connect to greetdeez rpc: %w", err)
	}
	if timeout <= 0 {
		timeout = defaultCallTimeout
	}
	return &SocketClient{conn: conn, timeout: timeout}, nil
}

func (c *SocketClient) Close() error {
	if c == nil || c.conn == nil {
		return nil
	}
	return c.conn.Close()
}

// Call invokes method (e.g. "greetdeez.v1.GreeterService/Login") and
// unmarshals the reply into resp.
func (c *SocketClient) Call(ctx context.Context, method string, req, resp proto.Message) error {
	payload, err := proto.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}
	raw, err := proto.Marshal(&pb.RpcEnvelope{Method: method, Payload: payload})
	if err != nil {
		return fmt.Errorf("marshal envelope: %w", err)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(c.timeout)
	}
	if err := c.conn.SetDeadline(deadline); err != nil {
		return fmt.Errorf("set deadline: %w", err)
	}
	defer c.conn.SetDeadline(time.Time{}) //nolint:errcheck

	if err := writeFrame(c.conn, raw); err != nil {
		return fmt.Errorf("write frame: %w", err)
	}
	replyRaw, err := readFrame(c.conn)
	if err != nil {
		return fmt.Errorf("read frame: %w", err)
	}

	var result pb.RpcResult
	if err := proto.Unmarshal(replyRaw, &result); err != nil {
		return fmt.Errorf("unmarshal result: %w", err)
	}
	if result.Error != "" {
		return errors.New(result.Error)
	}
	if err := proto.Unmarshal(result.Payload, resp); err != nil {
		return fmt.Errorf("unmarshal response: %w", err)
	}
	return nil
}
