// Package greetdtest fakes greetd socket for tests
package greetdtest

import (
	"encoding/binary"
	"encoding/json"
	"io"
	"net"
	"path/filepath"
	"testing"
)

// Handler maps one request to one scripted response
type Handler func(req map[string]any) map[string]any

// Start runs fake greetd returns socket path cleans up itself
func Start(t testing.TB, handler Handler) string {
	t.Helper()

	sock := filepath.Join(t.TempDir(), "greetd.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { ln.Close() })

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go serve(conn, handler)
		}
	}()
	return sock
}

func serve(conn net.Conn, handler Handler) {
	defer conn.Close()
	for {
		req, err := read(conn)
		if err != nil {
			return
		}
		if err := write(conn, handler(req)); err != nil {
			return
		}
	}
}

func read(conn net.Conn) (map[string]any, error) {
	lenBuf := make([]byte, 4)
	if _, err := io.ReadFull(conn, lenBuf); err != nil {
		return nil, err
	}
	data := make([]byte, binary.LittleEndian.Uint32(lenBuf))
	if _, err := io.ReadFull(conn, data); err != nil {
		return nil, err
	}
	var req map[string]any
	if err := json.Unmarshal(data, &req); err != nil {
		return nil, err
	}
	return req, nil
}

func write(conn net.Conn, msg map[string]any) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	lenBuf := make([]byte, 4)
	binary.LittleEndian.PutUint32(lenBuf, uint32(len(data)))
	if _, err := conn.Write(lenBuf); err != nil {
		return err
	}
	_, err = conn.Write(data)
	return err
}
