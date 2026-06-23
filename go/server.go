package protocomm

import (
	"io"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

type Interceptor func(ctx *ServerContext, code StatusCode, req, resp []byte, dur time.Duration)

type Peer struct {
	id      uint64
	addr    string
	conn    net.Conn
	writeMu sync.Mutex
}

func (p *Peer) ID() uint64          { return p.id }
func (p *Peer) PeerAddress() string { return p.addr }

func (p *Peer) Send(methodID uint32, payload []byte) error {
	hdr := FrameHeader{
		Type:        FramePush,
		MethodID:    methodID,
		PayloadSize: uint32(len(payload)),
	}
	p.writeMu.Lock()
	defer p.writeMu.Unlock()
	return WriteFrame(p.conn, hdr, payload)
}

func (p *Peer) Close() error { return p.conn.Close() }

type Server struct {
	listener        net.Listener
	handshakeHeader string

	handlersMu sync.RWMutex
	handlers   map[uint32]MethodHandler

	sessionsMu sync.Mutex
	sessions   map[uint64]*Peer
	nextConnID atomic.Uint64

	onConnect    func(*Peer)
	onDisconnect func(*Peer)
	onHandshake  func(*Peer, string) bool
	interceptor  Interceptor

	sem  chan struct{}
	done chan struct{}
	wg   sync.WaitGroup
}

func (s *Server) RegisterMethod(methodID uint32, handler MethodHandler) {
	s.handlersMu.Lock()
	s.handlers[methodID] = handler
	s.handlersMu.Unlock()
}

func (s *Server) Port() int {
	return s.listener.Addr().(*net.TCPAddr).Port
}

func (s *Server) GetConnections() []*Peer {
	s.sessionsMu.Lock()
	defer s.sessionsMu.Unlock()
	result := make([]*Peer, 0, len(s.sessions))
	for _, p := range s.sessions {
		result = append(result, p)
	}
	return result
}

func (s *Server) Broadcast(methodID uint32, payload []byte) {
	for _, p := range s.GetConnections() {
		_ = p.Send(methodID, payload)
	}
}

func (s *Server) Wait() { s.wg.Wait() }

func (s *Server) Shutdown() {
	select {
	case <-s.done:
		return
	default:
		close(s.done)
	}
	s.listener.Close()
	s.sessionsMu.Lock()
	for _, p := range s.sessions {
		p.conn.Close()
	}
	s.sessionsMu.Unlock()
	s.wg.Wait()
}

func (s *Server) acceptLoop() {
	defer s.wg.Done()
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.done:
				return
			default:
				continue
			}
		}
		if tcp, ok := conn.(*net.TCPConn); ok {
			tcp.SetNoDelay(true)
		}
		id := s.nextConnID.Add(1) - 1
		peer := &Peer{id: id, addr: conn.RemoteAddr().String(), conn: conn}

		s.sessionsMu.Lock()
		s.sessions[id] = peer
		s.sessionsMu.Unlock()

		s.wg.Add(1)
		go s.handleConn(peer)
	}
}

func (s *Server) handleConn(peer *Peer) {
	var inflight sync.WaitGroup
	defer func() {
		inflight.Wait()
		if s.onDisconnect != nil {
			s.onDisconnect(peer)
		}
		s.sessionsMu.Lock()
		delete(s.sessions, peer.id)
		s.sessionsMu.Unlock()
		peer.conn.Close()
		s.wg.Done()
	}()

	if s.onConnect != nil {
		s.onConnect(peer)
	}

	if s.handshakeHeader != "" {
		expected := []byte(s.handshakeHeader)
		received := make([]byte, len(expected))
		if _, err := io.ReadFull(peer.conn, received); err != nil {
			return
		}
		if string(received) != s.handshakeHeader {
			return
		}
		if s.onHandshake != nil && !s.onHandshake(peer, string(received)) {
			return
		}
		if _, err := peer.conn.Write(expected); err != nil {
			return
		}
	}

	for {
		hdr, payload, err := ReadFrame(peer.conn)
		if err != nil {
			return
		}
		if hdr.Type != FrameRequest {
			return
		}

		if s.sem != nil {
			select {
			case s.sem <- struct{}{}:
			case <-s.done:
				return
			}
		}
		inflight.Add(1)
		go func(hdr FrameHeader, payload []byte) {
			defer inflight.Done()
			if s.sem != nil {
				defer func() { <-s.sem }()
			}
			s.serveRequest(peer, hdr, payload)
		}(hdr, payload)
	}
}

func (s *Server) serveRequest(peer *Peer, hdr FrameHeader, payload []byte) {
	ctx := &ServerContext{
		PeerAddress: peer.addr,
		MethodID:    hdr.MethodID,
		CallID:      hdr.CallID,
		Trace:       s.interceptor != nil,
	}

	var respPayload []byte
	var respCode StatusCode

	start := time.Now()
	s.handlersMu.RLock()
	handler, ok := s.handlers[hdr.MethodID]
	s.handlersMu.RUnlock()

	if !ok {
		respCode = Unimplemented
	} else {
		var st Status
		respPayload, st = handler(ctx, payload)
		if !st.IsOK() {
			respCode = st.Code
			respPayload = nil
		}
	}

	if s.interceptor != nil {
		s.interceptor(ctx, respCode, payload, respPayload, time.Since(start))
	}

	respHdr := FrameHeader{
		Type:        FrameResponse,
		StatusCode:  uint16(respCode),
		CallID:      hdr.CallID,
		MethodID:    hdr.MethodID,
		PayloadSize: uint32(len(respPayload)),
	}

	peer.writeMu.Lock()
	err := WriteFrame(peer.conn, respHdr, respPayload)
	peer.writeMu.Unlock()
	if err != nil {
		peer.conn.Close()
	}
}

type ServerBuilder struct {
	address         string
	port            uint16
	handshakeHeader string
	services        []Service
	onConnect       func(*Peer)
	onDisconnect    func(*Peer)
	onHandshake     func(*Peer, string) bool
	interceptor     Interceptor
	maxConcurrent   int
}

func NewServerBuilder() *ServerBuilder {
	return &ServerBuilder{
		address:         "0.0.0.0",
		handshakeHeader: "pc1",
		maxConcurrent:   256,
	}
}

func (b *ServerBuilder) AddListeningPort(address string, port uint16) *ServerBuilder {
	b.address = address
	b.port = port
	return b
}

func (b *ServerBuilder) SetHandshakeHeader(header string) *ServerBuilder {
	b.handshakeHeader = header
	return b
}

func (b *ServerBuilder) RegisterService(svc Service) *ServerBuilder {
	b.services = append(b.services, svc)
	return b
}

func (b *ServerBuilder) SetOnConnect(cb func(*Peer)) *ServerBuilder {
	b.onConnect = cb
	return b
}

func (b *ServerBuilder) SetOnDisconnect(cb func(*Peer)) *ServerBuilder {
	b.onDisconnect = cb
	return b
}

func (b *ServerBuilder) SetOnHandshake(cb func(*Peer, string) bool) *ServerBuilder {
	b.onHandshake = cb
	return b
}

func (b *ServerBuilder) SetMaxConcurrentRequests(n int) *ServerBuilder {
	b.maxConcurrent = n
	return b
}

func (b *ServerBuilder) SetInterceptor(fn Interceptor) *ServerBuilder {
	b.interceptor = fn
	return b
}

func (b *ServerBuilder) BuildAndStart() (*Server, error) {
	s := &Server{
		handlers:        make(map[uint32]MethodHandler),
		sessions:        make(map[uint64]*Peer),
		handshakeHeader: b.handshakeHeader,
		onConnect:       b.onConnect,
		onDisconnect:    b.onDisconnect,
		onHandshake:     b.onHandshake,
		interceptor:     b.interceptor,
		done:            make(chan struct{}),
	}
	s.nextConnID.Store(1)
	if b.maxConcurrent > 0 {
		s.sem = make(chan struct{}, b.maxConcurrent)
	}

	for _, svc := range b.services {
		svc.RegisterWith(s)
	}

	ln, err := net.Listen("tcp", net.JoinHostPort(b.address, strconv.Itoa(int(b.port))))
	if err != nil {
		return nil, err
	}
	s.listener = ln

	s.wg.Add(1)
	go s.acceptLoop()

	return s, nil
}
