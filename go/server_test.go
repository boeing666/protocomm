package protocomm_test

import (
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/boeing666/protocomm/go"
	mathpb "github.com/boeing666/protocomm/go/proto/math"
)

// funcService registers arbitrary handlers, so a test can build its own server.
type funcService map[uint32]protocomm.MethodHandler

func (s funcService) RegisterWith(srv *protocomm.Server) {
	for id, h := range s {
		srv.RegisterMethod(id, h)
	}
}

var echoID = protocomm.MethodID("test", "Echo", "Echo")

func echo(_ *protocomm.ServerContext, req []byte) ([]byte, protocomm.Status) {
	return req, protocomm.StatusOK()
}

// startServer builds a server on a free local port and shuts it down at the end of the test.
func startServer(t *testing.T, b *protocomm.ServerBuilder) *protocomm.Server {
	t.Helper()
	srv, err := b.AddListeningPort("127.0.0.1", 0).BuildAndStart()
	if err != nil {
		t.Fatalf("server start: %v", err)
	}
	t.Cleanup(srv.Shutdown)
	return srv
}

func dial(t *testing.T, srv *protocomm.Server, cfg ...protocomm.ChannelConfig) *protocomm.Channel {
	t.Helper()
	ch := protocomm.CreateChannel("127.0.0.1", uint16(srv.Port()), cfg...)
	t.Cleanup(func() { ch.Close() })
	return ch
}

// waitFor polls cond until it is true or the timeout expires.
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestServerConnectionCallbacks(t *testing.T) {
	var (
		mu                    sync.Mutex
		connected, handshaken []uint64
		disconnected          []uint64
		addr, handshakeHeader string
	)
	srv := startServer(t, protocomm.NewServerBuilder().
		RegisterService(funcService{echoID: echo}).
		SetOnConnect(func(p *protocomm.Peer) {
			mu.Lock()
			connected = append(connected, p.ID())
			addr = p.PeerAddress()
			mu.Unlock()
		}).
		SetOnHandshake(func(p *protocomm.Peer, header string) bool {
			mu.Lock()
			handshaken = append(handshaken, p.ID())
			handshakeHeader = header
			mu.Unlock()
			return true
		}).
		SetOnDisconnect(func(p *protocomm.Peer) {
			mu.Lock()
			disconnected = append(disconnected, p.ID())
			mu.Unlock()
		}))

	ch := dial(t, srv)
	if st := ch.Connect(); !st.IsOK() {
		t.Fatalf("Connect: %v", st)
	}
	if _, st := ch.UnaryCall(echoID, []byte("ping")); !st.IsOK() {
		t.Fatalf("call: %v", st)
	}
	if n := len(srv.GetConnections()); n != 1 {
		t.Errorf("GetConnections: got %d peers, want 1", n)
	}

	ch.Close()
	waitFor(t, "OnDisconnect", func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(disconnected) == 1
	})
	waitFor(t, "peer removed", func() bool { return len(srv.GetConnections()) == 0 })

	mu.Lock()
	defer mu.Unlock()
	if len(connected) != 1 || len(handshaken) != 1 {
		t.Fatalf("callbacks: connect=%v handshake=%v", connected, handshaken)
	}
	if connected[0] != handshaken[0] || connected[0] != disconnected[0] {
		t.Errorf("peer IDs differ between callbacks: %v %v %v", connected, handshaken, disconnected)
	}
	if !strings.HasPrefix(addr, "127.0.0.1:") {
		t.Errorf("PeerAddress = %q, want 127.0.0.1:<port>", addr)
	}
	if handshakeHeader != "pc1" {
		t.Errorf("handshake header = %q, want pc1", handshakeHeader)
	}
}

func TestServerPeerIDsAreUnique(t *testing.T) {
	var (
		mu  sync.Mutex
		ids = map[uint64]bool{}
	)
	srv := startServer(t, protocomm.NewServerBuilder().
		RegisterService(funcService{echoID: echo}).
		SetOnConnect(func(p *protocomm.Peer) {
			mu.Lock()
			ids[p.ID()] = true
			mu.Unlock()
		}))

	for i := 0; i < 3; i++ {
		if st := dial(t, srv).Connect(); !st.IsOK() {
			t.Fatalf("Connect #%d: %v", i, st)
		}
	}
	waitFor(t, "3 peers", func() bool { return len(srv.GetConnections()) == 3 })

	mu.Lock()
	defer mu.Unlock()
	if len(ids) != 3 {
		t.Errorf("got %d distinct peer IDs, want 3: %v", len(ids), ids)
	}
	if ids[0] {
		t.Error("peer ID 0 is used; IDs are expected to start at 1")
	}
}

func TestServerHandshakeRejected(t *testing.T) {
	srv := startServer(t, protocomm.NewServerBuilder().
		RegisterService(funcService{echoID: echo}).
		SetOnHandshake(func(*protocomm.Peer, string) bool { return false }))

	st := dial(t, srv).Connect()
	if st.IsOK() {
		t.Fatal("Connect succeeded although OnHandshake rejected the peer")
	}
	if st.Code != protocomm.Unavailable {
		t.Errorf("got %v, want UNAVAILABLE", st)
	}
	waitFor(t, "rejected peer removed", func() bool { return len(srv.GetConnections()) == 0 })
}

func TestServerBroadcastPush(t *testing.T) {
	const pushID uint32 = 0xC0FFEE
	srv := startServer(t, protocomm.NewServerBuilder().RegisterService(funcService{echoID: echo}))

	type push struct {
		method  uint32
		payload string
	}
	const clients = 3
	got := make(chan push, clients)
	for i := 0; i < clients; i++ {
		ch := dial(t, srv, protocomm.ChannelConfig{
			OnPush: func(methodID uint32, payload []byte) {
				got <- push{methodID, string(payload)}
			},
		})
		if st := ch.Connect(); !st.IsOK() {
			t.Fatalf("Connect: %v", st)
		}
	}
	waitFor(t, "all clients connected", func() bool { return len(srv.GetConnections()) == clients })

	srv.Broadcast(pushID, []byte("news"))

	for i := 0; i < clients; i++ {
		select {
		case p := <-got:
			if p.method != pushID || p.payload != "news" {
				t.Errorf("push #%d: got %+v", i, p)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("only %d of %d clients received the push", i, clients)
		}
	}
}

func TestPushWithoutHandlerIsIgnored(t *testing.T) {
	srv := startServer(t, protocomm.NewServerBuilder().RegisterService(funcService{echoID: echo}))
	ch := dial(t, srv)
	if st := ch.Connect(); !st.IsOK() {
		t.Fatalf("Connect: %v", st)
	}
	waitFor(t, "peer", func() bool { return len(srv.GetConnections()) == 1 })

	if err := srv.GetConnections()[0].Send(1, []byte("unused")); err != nil {
		t.Fatalf("Send: %v", err)
	}
	// The connection must stay usable after an unhandled push.
	resp, st := ch.UnaryCall(echoID, []byte("still alive"))
	if !st.IsOK() || string(resp) != "still alive" {
		t.Fatalf("call after push: %q %v", resp, st)
	}
}

func TestPeerCloseFailsPendingCall(t *testing.T) {
	var (
		started = make(chan struct{})
		release = make(chan struct{})
	)
	blockID := protocomm.MethodID("test", "Echo", "Block")
	srv := startServer(t, protocomm.NewServerBuilder().
		RegisterService(funcService{blockID: func(*protocomm.ServerContext, []byte) ([]byte, protocomm.Status) {
			close(started)
			<-release
			return nil, protocomm.StatusOK()
		}}))
	defer close(release)

	ch := dial(t, srv)
	done := make(chan protocomm.Status, 1)
	ch.AsyncUnaryCall(blockID, nil, func(st protocomm.Status, _ []byte) { done <- st })

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("handler did not start")
	}
	srv.GetConnections()[0].Close()

	select {
	case st := <-done:
		if st.Code != protocomm.Unavailable {
			t.Errorf("pending call: got %v, want UNAVAILABLE", st)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("pending call was not failed after the connection dropped")
	}
}

func TestChannelReconnectsAfterDrop(t *testing.T) {
	srv := startServer(t, protocomm.NewServerBuilder().RegisterService(funcService{echoID: echo}))
	ch := dial(t, srv)
	if _, st := ch.UnaryCall(echoID, []byte("one")); !st.IsOK() {
		t.Fatalf("first call: %v", st)
	}

	srv.GetConnections()[0].Close()
	waitFor(t, "old peer removed", func() bool { return len(srv.GetConnections()) == 0 })
	// Give the client read loop a moment to notice the drop.
	time.Sleep(20 * time.Millisecond)

	resp, st := ch.UnaryCall(echoID, []byte("two"))
	if !st.IsOK() || string(resp) != "two" {
		t.Fatalf("call after reconnect: %q %v", resp, st)
	}
}

func TestMaxConcurrentRequests(t *testing.T) {
	var running, peak atomic.Int32
	slowID := protocomm.MethodID("test", "Echo", "Slow")
	srv := startServer(t, protocomm.NewServerBuilder().
		SetMaxConcurrentRequests(1).
		RegisterService(funcService{slowID: func(*protocomm.ServerContext, []byte) ([]byte, protocomm.Status) {
			n := running.Add(1)
			for {
				p := peak.Load()
				if n <= p || peak.CompareAndSwap(p, n) {
					break
				}
			}
			time.Sleep(20 * time.Millisecond)
			running.Add(-1)
			return nil, protocomm.StatusOK()
		}}))

	ch := dial(t, srv)
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, st := ch.UnaryCall(slowID, nil); !st.IsOK() {
				t.Errorf("call: %v", st)
			}
		}()
	}
	wg.Wait()

	if p := peak.Load(); p != 1 {
		t.Errorf("peak concurrent handlers = %d, want 1", p)
	}
}

func TestChannelConnectAsync(t *testing.T) {
	srv := startServer(t, protocomm.NewServerBuilder().RegisterService(funcService{echoID: echo}))
	ch := dial(t, srv)

	done := make(chan protocomm.Status, 1)
	ch.ConnectAsync(func(st protocomm.Status) { done <- st })
	select {
	case st := <-done:
		if !st.IsOK() {
			t.Fatalf("ConnectAsync: %v", st)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("ConnectAsync callback was not called")
	}
	// A second Connect on a live channel is a no-op.
	if st := ch.Connect(); !st.IsOK() {
		t.Errorf("repeated Connect: %v", st)
	}
}

func TestChannelConnectRefused(t *testing.T) {
	srv := startServer(t, protocomm.NewServerBuilder())
	port := uint16(srv.Port())
	srv.Shutdown()

	ch := protocomm.CreateChannel("127.0.0.1", port)
	defer ch.Close()
	if st := ch.Connect(); st.Code != protocomm.Unavailable {
		t.Errorf("Connect to closed port: got %v, want UNAVAILABLE", st)
	}
}

func TestChannelClosed(t *testing.T) {
	srv := startServer(t, protocomm.NewServerBuilder().RegisterService(funcService{echoID: echo}))
	ch := protocomm.CreateChannel("127.0.0.1", uint16(srv.Port()))
	if err := ch.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := ch.Close(); err != nil {
		t.Errorf("second Close: %v", err)
	}
	if st := ch.Connect(); st.Code != protocomm.Unavailable {
		t.Errorf("Connect after Close: got %v, want UNAVAILABLE", st)
	}
	if _, st := ch.UnaryCall(echoID, nil); st.IsOK() {
		t.Error("call on a closed channel succeeded")
	}
}

func TestClientInterceptor(t *testing.T) {
	type trace struct {
		method, req, resp string
		st                protocomm.Status
	}
	srv := startServer(t, protocomm.NewServerBuilder().
		RegisterService(mathpb.NewCalculatorService(calcImpl{})))
	ch := dial(t, srv)

	client := mathpb.NewCalculatorClient(ch)
	if ch.Tracing() {
		t.Fatal("Tracing() is true before SetInterceptor")
	}
	// Without an interceptor Trace must be a no-op.
	ch.Trace("x", "", "", protocomm.StatusOK())

	var traces []trace
	ch.SetInterceptor(func(method, req, resp string, st protocomm.Status) {
		traces = append(traces, trace{method, req, resp, st})
	})
	if !ch.Tracing() {
		t.Fatal("Tracing() is false after SetInterceptor")
	}

	if _, st := client.Add(&mathpb.CalcRequest{A: 2, B: 3}); !st.IsOK() {
		t.Fatalf("Add: %v", st)
	}
	// calcImpl does not implement Divide, so the call fails on the server.
	if _, st := client.Divide(&mathpb.CalcRequest{A: 1, B: 0}); st.Code != protocomm.Unimplemented {
		t.Fatalf("Divide: got %v, want UNIMPLEMENTED", st)
	}

	if len(traces) != 2 {
		t.Fatalf("got %d traces, want 2: %+v", len(traces), traces)
	}
	ok, failed := traces[0], traces[1]
	if ok.method != "Calculator.Add" || !strings.Contains(ok.req, "a:2") || !strings.Contains(ok.resp, "result:5") || !ok.st.IsOK() {
		t.Errorf("successful call trace: %+v", ok)
	}
	if failed.method != "Calculator.Divide" || failed.resp != "" || failed.st.Code != protocomm.Unimplemented {
		t.Errorf("failed call trace: %+v", failed)
	}
}

func TestServerWaitReturnsAfterShutdown(t *testing.T) {
	srv, err := protocomm.NewServerBuilder().AddListeningPort("127.0.0.1", 0).BuildAndStart()
	if err != nil {
		t.Fatal(err)
	}
	waited := make(chan struct{})
	go func() {
		srv.Wait()
		close(waited)
	}()

	select {
	case <-waited:
		t.Fatal("Wait returned while the server is running")
	case <-time.After(50 * time.Millisecond):
	}
	srv.Shutdown()
	srv.Shutdown() // second call must not panic
	select {
	case <-waited:
	case <-time.After(2 * time.Second):
		t.Fatal("Wait did not return after Shutdown")
	}
}
