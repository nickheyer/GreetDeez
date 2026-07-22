package rpc

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
)

// Unix socket transport for the greetdeez protocol. Same envelope the webview
// bridge speaks, minus the webview: any process that can open the socket can
// drive the greeter, in any language with protobuf support.
//
// Wire format both directions is a u32 little-endian frame length followed by
// that many bytes. Request frames carry transport.v1.RpcEnvelope, response
// frames transport.v1.RpcResult, encoded as binary protobuf. One request per
// connection is answered at a time, in order.

// same cap greetd uses requests are tiny
const maxFrameLen = 1 << 20

type SocketServer struct {
	ln   net.Listener
	path string
}

// ServeSocket listens on path and answers protocol calls via d.
// The dispatcher should be a non-debug one so the wire stays binary protobuf.
func ServeSocket(path string, d *Dispatcher) (*SocketServer, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("socket dir: %w", err)
	}
	// stale socket from a previous run
	_ = os.Remove(path)

	ln, err := net.Listen("unix", path)
	if err != nil {
		return nil, fmt.Errorf("listen %s: %w", path, err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		ln.Close()
		return nil, fmt.Errorf("chmod socket: %w", err)
	}

	s := &SocketServer{ln: ln, path: path}
	go s.accept(d)
	slog.Info("rpc socket listening", "path", path)
	return s, nil
}

func (s *SocketServer) Path() string { return s.path }

func (s *SocketServer) Close() error {
	err := s.ln.Close()
	_ = os.Remove(s.path)
	return err
}

func (s *SocketServer) accept(d *Dispatcher) {
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			if !errors.Is(err, net.ErrClosed) {
				slog.Warn("rpc socket accept", "error", err)
			}
			return
		}
		go serveConn(conn, d)
	}
}

func serveConn(conn net.Conn, d *Dispatcher) {
	defer conn.Close()
	for {
		raw, err := readFrame(conn)
		if err != nil {
			if !errors.Is(err, io.EOF) && !errors.Is(err, net.ErrClosed) {
				slog.Debug("rpc socket read", "error", err)
			}
			return
		}
		if err := writeFrame(conn, d.DispatchRaw(raw)); err != nil {
			slog.Debug("rpc socket write", "error", err)
			return
		}
	}
}

func readFrame(r io.Reader) ([]byte, error) {
	var lenBuf [4]byte
	if _, err := io.ReadFull(r, lenBuf[:]); err != nil {
		return nil, err
	}
	length := binary.LittleEndian.Uint32(lenBuf[:])
	if length > maxFrameLen {
		return nil, fmt.Errorf("frame too large: %d bytes", length)
	}
	data := make([]byte, length)
	if _, err := io.ReadFull(r, data); err != nil {
		return nil, err
	}
	return data, nil
}

func writeFrame(w io.Writer, data []byte) error {
	var lenBuf [4]byte
	binary.LittleEndian.PutUint32(lenBuf[:], uint32(len(data)))
	if _, err := w.Write(lenBuf[:]); err != nil {
		return err
	}
	_, err := w.Write(data)
	return err
}
