package rpc

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	pb "github.com/nickheyer/greetdeez/gen/go/transport/v1"
)

type HandlerFunc func(ctx context.Context, payload []byte) (proto.Message, error)

type Dispatcher struct {
	handlers map[string]HandlerFunc
	debug    bool
}

func NewDispatcher(debug bool) *Dispatcher {
	return &Dispatcher{handlers: make(map[string]HandlerFunc), debug: debug}
}

func (d *Dispatcher) Register(method string, h HandlerFunc) {
	d.handlers[method] = h
}

func (d *Dispatcher) Marshal(m proto.Message) ([]byte, error) {
	if d.debug {
		return protojson.Marshal(m)
	}
	return proto.Marshal(m)
}

func (d *Dispatcher) Unmarshal(b []byte, m proto.Message) error {
	if d.debug {
		return protojson.Unmarshal(b, m)
	}
	return proto.Unmarshal(b, m)
}

// DispatchRaw takes an encoded RpcEnvelope and returns an encoded RpcResult.
// Every transport (webview bridge, unix socket) funnels through here.
func (d *Dispatcher) DispatchRaw(raw []byte) []byte {
	var env pb.RpcEnvelope
	if err := d.Unmarshal(raw, &env); err != nil {
		return d.resultBytes(nil, fmt.Errorf("envelope: %w", err))
	}
	h, ok := d.handlers[env.Method]
	if !ok {
		return d.resultBytes(nil, fmt.Errorf("unknown method: %s", env.Method))
	}
	resp, err := h(context.Background(), env.Payload)
	if err != nil {
		return d.resultBytes(nil, err)
	}
	respBytes, err := d.Marshal(resp)
	if err != nil {
		return d.resultBytes(nil, fmt.Errorf("marshal: %w", err))
	}
	return d.resultBytes(respBytes, nil)
}

func (d *Dispatcher) WebViewHandler() func(string) string {
	return func(input string) string {
		raw, err := base64.StdEncoding.DecodeString(input)
		if err != nil {
			return base64.StdEncoding.EncodeToString(d.resultBytes(nil, fmt.Errorf("base64: %w", err)))
		}
		return base64.StdEncoding.EncodeToString(d.DispatchRaw(raw))
	}
}

func (d *Dispatcher) resultBytes(payload []byte, err error) []byte {
	result := &pb.RpcResult{}
	if err != nil {
		result.Error = err.Error()
		slog.Warn("rpc error", "error", err)
	} else {
		result.Payload = payload
	}
	b, e := d.Marshal(result)
	if e != nil {
		slog.Error("failed to marshal pb.RpcResult", "error", e)
		return nil
	}
	return b
}

// Handle is called by generated registration code.
func Handle[Req any, ReqPtr interface {
	*Req
	proto.Message
}, Resp proto.Message](
	d *Dispatcher, method string, fn func(context.Context, ReqPtr) (Resp, error),
) {
	d.Register(method, func(ctx context.Context, payload []byte) (proto.Message, error) {
		req := ReqPtr(new(Req))
		if err := d.Unmarshal(payload, req); err != nil {
			return nil, fmt.Errorf("unmarshal %s: %w", method, err)
		}
		return fn(ctx, req)
	})
}
