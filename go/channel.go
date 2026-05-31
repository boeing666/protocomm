package protocomm

import (
	"io"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
)

type ChannelConfig struct {
	HandshakeHeader string
	OnPush          func(methodID uint32, payload []byte)
}

type Channel struct {
	host       string
	port       uint16
	config     ChannelConfig
	conn       net.Conn
	connected  bool
	mu         sync.Mutex
	nextCallID atomic.Uint32
}

func CreateChannel(host string, port uint16, config ...ChannelConfig) *Channel {
	ch := &Channel{host: host, port: port}
	if len(config) > 0 {
		ch.config = config[0]
	}
	if ch.config.HandshakeHeader == "" {
		ch.config.HandshakeHeader = "pc1"
	}
	ch.nextCallID.Store(1)
	return ch
}

func (c *Channel) ensureConnected() Status {
	if c.connected {
		return StatusOK()
	}
	conn, err := net.Dial("tcp", net.JoinHostPort(c.host, strconv.Itoa(int(c.port))))
	if err != nil {
		return Status{Code: Unavailable, Message: "connect: " + err.Error()}
	}
	if tcp, ok := conn.(*net.TCPConn); ok {
		tcp.SetNoDelay(true)
	}
	c.conn = conn

	if c.config.HandshakeHeader != "" {
		hdr := []byte(c.config.HandshakeHeader)
		if _, err := conn.Write(hdr); err != nil {
			conn.Close()
			c.conn = nil
			return Status{Code: Unavailable, Message: "handshake write: " + err.Error()}
		}
		received := make([]byte, len(hdr))
		if _, err := io.ReadFull(conn, received); err != nil {
			conn.Close()
			c.conn = nil
			return Status{Code: Unavailable, Message: "handshake read: " + err.Error()}
		}
		if string(received) != c.config.HandshakeHeader {
			conn.Close()
			c.conn = nil
			return Status{Code: FailedPrecondition, Message: "handshake mismatch"}
		}
	}

	c.connected = true
	return StatusOK()
}

func (c *Channel) UnaryCall(methodID uint32, request []byte) ([]byte, Status) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if st := c.ensureConnected(); !st.IsOK() {
		return nil, st
	}

	callID := c.nextCallID.Add(1) - 1

	reqHdr := FrameHeader{
		Type:        FrameRequest,
		CallID:      callID,
		MethodID:    methodID,
		PayloadSize: uint32(len(request)),
	}
	if err := WriteFrame(c.conn, reqHdr, request); err != nil {
		c.connected = false
		return nil, Status{Code: Unavailable, Message: "write: " + err.Error()}
	}

	for {
		respHdr, respPayload, err := ReadFrame(c.conn)
		if err != nil {
			c.connected = false
			return nil, Status{Code: Unavailable, Message: "read: " + err.Error()}
		}

		if respHdr.Type == FramePush {
			if c.config.OnPush != nil {
				c.config.OnPush(respHdr.MethodID, respPayload)
			}
			continue
		}

		if respHdr.Type != FrameResponse {
			c.connected = false
			return nil, Status{Code: Internal, Message: "expected response frame"}
		}

		code := StatusCode(respHdr.StatusCode)
		if code != OK {
			return nil, Status{Code: code, Message: "remote error"}
		}
		return respPayload, StatusOK()
	}
}

func (c *Channel) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn != nil {
		err := c.conn.Close()
		c.conn = nil
		c.connected = false
		return err
	}
	return nil
}
