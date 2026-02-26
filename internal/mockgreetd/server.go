package mockgreetd

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"sync"
)

// Server is a mock greetd that speaks the real IPC protocol over a Unix socket.
// Useful for unit tests and local dev without real greetd/PAM.
type Server struct {
	Users    map[string]string // username -> password
	SockPath string
	listener net.Listener
	wg       sync.WaitGroup
	done     chan struct{}
}

type request struct {
	Type     string   `json:"type"`
	Username string   `json:"username,omitempty"`
	Response *string  `json:"response,omitempty"`
	Cmd      []string `json:"cmd,omitempty"`
}

type response struct {
	Type            string  `json:"type"`
	AuthMessageType *string `json:"auth_message_type,omitempty"`
	AuthMessage     *string `json:"auth_message,omitempty"`
	ErrorType       *string `json:"error_type,omitempty"`
	Description     *string `json:"description,omitempty"`
}

func ptr(s string) *string { return &s }

// New creates a mock greetd server with the given users and socket path.
func New(sockPath string, users map[string]string) *Server {
	return &Server{
		Users:    users,
		SockPath: sockPath,
		done:     make(chan struct{}),
	}
}

// Start begins listening and accepting connections.
func (s *Server) Start() error {
	os.Remove(s.SockPath)

	var err error
	s.listener, err = net.Listen("unix", s.SockPath)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		for {
			conn, err := s.listener.Accept()
			if err != nil {
				select {
				case <-s.done:
					return
				default:
					log.Printf("mockgreetd: accept: %v", err)
					continue
				}
			}
			s.wg.Add(1)
			go func() {
				defer s.wg.Done()
				s.handleConn(conn)
			}()
		}
	}()

	return nil
}

// Stop shuts down the server and cleans up the socket.
func (s *Server) Stop() {
	close(s.done)
	s.listener.Close()
	s.wg.Wait()
	os.Remove(s.SockPath)
}

func (s *Server) handleConn(conn net.Conn) {
	defer conn.Close()

	type state int
	const (
		stateIdle state = iota
		stateAwaitingAuth
		stateAuthenticated
	)

	st := stateIdle
	var currentUser string

	for {
		req, err := readMsg(conn)
		if err != nil {
			if err != io.EOF {
				log.Printf("mockgreetd: read: %v", err)
			}
			return
		}

		switch req.Type {
		case "create_session":
			if st != stateIdle {
				writeMsg(conn, response{
					Type:        "error",
					ErrorType:   ptr("error"),
					Description: ptr("session already active"),
				})
				continue
			}
			currentUser = req.Username
			if _, ok := s.Users[currentUser]; !ok {
				writeMsg(conn, response{
					Type:        "error",
					ErrorType:   ptr("auth_error"),
					Description: ptr("unknown user"),
				})
				st = stateIdle
				continue
			}
			st = stateAwaitingAuth
			writeMsg(conn, response{
				Type:            "auth_message",
				AuthMessageType: ptr("secret"),
				AuthMessage:     ptr("Password: "),
			})

		case "post_auth_message_response":
			if st != stateAwaitingAuth {
				writeMsg(conn, response{
					Type:        "error",
					ErrorType:   ptr("error"),
					Description: ptr("no auth in progress"),
				})
				continue
			}
			expected := s.Users[currentUser]
			given := ""
			if req.Response != nil {
				given = *req.Response
			}
			if given == expected {
				st = stateAuthenticated
				writeMsg(conn, response{Type: "success"})
			} else {
				st = stateIdle
				currentUser = ""
				writeMsg(conn, response{
					Type:        "error",
					ErrorType:   ptr("auth_error"),
					Description: ptr("authentication failed"),
				})
			}

		case "start_session":
			if st != stateAuthenticated {
				writeMsg(conn, response{
					Type:        "error",
					ErrorType:   ptr("error"),
					Description: ptr("not authenticated"),
				})
				continue
			}
			log.Printf("mockgreetd: starting session for %s: %v", currentUser, req.Cmd)
			st = stateIdle
			currentUser = ""
			writeMsg(conn, response{Type: "success"})

		case "cancel_session":
			st = stateIdle
			currentUser = ""
			writeMsg(conn, response{Type: "success"})

		default:
			writeMsg(conn, response{
				Type:        "error",
				ErrorType:   ptr("error"),
				Description: ptr("unknown request type: " + req.Type),
			})
		}
	}
}

func readMsg(conn net.Conn) (*request, error) {
	lenBuf := make([]byte, 4)
	if _, err := io.ReadFull(conn, lenBuf); err != nil {
		return nil, err
	}
	length := binary.LittleEndian.Uint32(lenBuf)

	data := make([]byte, length)
	if _, err := io.ReadFull(conn, data); err != nil {
		return nil, err
	}

	var req request
	if err := json.Unmarshal(data, &req); err != nil {
		return nil, err
	}
	return &req, nil
}

func writeMsg(conn net.Conn, msg response) error {
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
