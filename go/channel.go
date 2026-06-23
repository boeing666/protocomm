package protocomm

import (
	"io"
	"net"
	"strconv"
	"sync"
)

type ChannelConfig struct {
	HandshakeHeader string
	OnPush          func(methodID uint32, payload []byte)
}

type ClientInterceptor func(method, requestText, responseText string, status Status)

type result struct {
	st      Status
	payload []byte
}

type Channel struct {
	host        string
	port        uint16
	config      ChannelConfig
	interceptor ClientInterceptor

	connectMu sync.Mutex
	writeMu   sync.Mutex

	mu         sync.Mutex
	conn       net.Conn
	connected  bool
	closed     bool
	gen        uint64
	nextCallID uint32
	pending    map[uint32]func(Status, []byte)
}

func CreateChannel(host string, port uint16, config ...ChannelConfig) *Channel {
	ch := &Channel{
		host:       host,
		port:       port,
		nextCallID: 1,
		pending:    make(map[uint32]func(Status, []byte)),
	}
	if len(config) > 0 {
		ch.config = config[0]
	}
	if ch.config.HandshakeHeader == "" {
		ch.config.HandshakeHeader = "pc1"
	}
	return ch
}

func (c *Channel) ensureConnected() Status {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return Status{Code: Unavailable, Message: "channel closed"}
	}
	if c.connected && c.conn != nil {
		c.mu.Unlock()
		return StatusOK()
	}
	c.mu.Unlock()

	c.connectMu.Lock()
	defer c.connectMu.Unlock()

	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return Status{Code: Unavailable, Message: "channel closed"}
	}
	if c.connected && c.conn != nil {
		c.mu.Unlock()
		return StatusOK()
	}
	c.mu.Unlock()

	conn, err := net.Dial("tcp", net.JoinHostPort(c.host, strconv.Itoa(int(c.port))))
	if err != nil {
		return Status{Code: Unavailable, Message: "connect: " + err.Error()}
	}
	if tcp, ok := conn.(*net.TCPConn); ok {
		tcp.SetNoDelay(true)
	}

	if c.config.HandshakeHeader != "" {
		hdr := []byte(c.config.HandshakeHeader)
		if _, err := conn.Write(hdr); err != nil {
			conn.Close()
			return Status{Code: Unavailable, Message: "handshake write: " + err.Error()}
		}
		received := make([]byte, len(hdr))
		if _, err := io.ReadFull(conn, received); err != nil {
			conn.Close()
			return Status{Code: Unavailable, Message: "handshake read: " + err.Error()}
		}
		if string(received) != c.config.HandshakeHeader {
			conn.Close()
			return Status{Code: FailedPrecondition, Message: "handshake mismatch"}
		}
	}

	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		conn.Close()
		return Status{Code: Unavailable, Message: "channel closed"}
	}
	c.gen++
	gen := c.gen
	c.conn = conn
	c.connected = true
	c.mu.Unlock()

	go c.readLoop(conn, gen)
	return StatusOK()
}

func (c *Channel) readLoop(conn net.Conn, gen uint64) {
	for {
		hdr, payload, err := ReadFrame(conn)
		if err != nil {
			c.teardown(gen, Status{Code: Unavailable, Message: "read: " + err.Error()})
			return
		}

		switch hdr.Type {
		case FramePush:
			if c.config.OnPush != nil {
				c.config.OnPush(hdr.MethodID, payload)
			}
			continue

		case FrameResponse:
			c.mu.Lock()
			cb := c.pending[hdr.CallID]
			if cb != nil {
				delete(c.pending, hdr.CallID)
			}
			c.mu.Unlock()
			if cb == nil {
				continue
			}
			code := StatusCode(hdr.StatusCode)
			if code != OK {
				cb(Status{Code: code, Message: "remote error"}, nil)
			} else {
				cb(StatusOK(), payload)
			}

		default:
			c.teardown(gen, Status{Code: Internal, Message: "unexpected frame type"})
			return
		}
	}
}

func (c *Channel) AsyncUnaryCall(methodID uint32, request []byte, cb func(Status, []byte)) {
	if st := c.ensureConnected(); !st.IsOK() {
		cb(st, nil)
		return
	}

	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		cb(Status{Code: Cancelled, Message: "channel closed"}, nil)
		return
	}
	if !c.connected || c.conn == nil {
		c.mu.Unlock()
		cb(Status{Code: Unavailable, Message: "not connected"}, nil)
		return
	}
	callID := c.nextCallID
	c.nextCallID++
	c.pending[callID] = cb
	conn := c.conn
	gen := c.gen
	c.mu.Unlock()

	reqHdr := FrameHeader{
		Type:        FrameRequest,
		CallID:      callID,
		MethodID:    methodID,
		PayloadSize: uint32(len(request)),
	}

	c.writeMu.Lock()
	err := WriteFrame(conn, reqHdr, request)
	c.writeMu.Unlock()

	if err != nil {
		c.teardown(gen, Status{Code: Unavailable, Message: "write: " + err.Error()})
	}
}

func (c *Channel) UnaryCall(methodID uint32, request []byte) ([]byte, Status) {
	done := make(chan result, 1)
	c.AsyncUnaryCall(methodID, request, func(st Status, payload []byte) {
		done <- result{st: st, payload: payload}
	})
	r := <-done
	return r.payload, r.st
}

func (c *Channel) Connect() Status {
	return c.ensureConnected()
}

func (c *Channel) ConnectAsync(cb func(Status)) {
	go func() {
		cb(c.ensureConnected())
	}()
}

func (c *Channel) SetInterceptor(fn ClientInterceptor) { c.interceptor = fn }
func (c *Channel) Tracing() bool                       { return c.interceptor != nil }
func (c *Channel) Trace(method, requestText, responseText string, status Status) {
	if c.interceptor != nil {
		c.interceptor(method, requestText, responseText, status)
	}
}

func (c *Channel) teardown(gen uint64, st Status) {
	c.mu.Lock()
	if gen != c.gen {
		c.mu.Unlock()
		return
	}
	c.gen++
	conn := c.conn
	c.conn = nil
	c.connected = false
	snapshot := c.pending
	c.pending = make(map[uint32]func(Status, []byte))
	c.mu.Unlock()

	if conn != nil {
		conn.Close()
	}
	for _, cb := range snapshot {
		cb(st, nil)
	}
}

func (c *Channel) Close() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	c.gen++
	conn := c.conn
	c.conn = nil
	c.connected = false
	snapshot := c.pending
	c.pending = make(map[uint32]func(Status, []byte))
	c.mu.Unlock()

	var err error
	if conn != nil {
		err = conn.Close()
	}
	for _, cb := range snapshot {
		cb(Status{Code: Cancelled, Message: "channel closed"}, nil)
	}
	return err
}
