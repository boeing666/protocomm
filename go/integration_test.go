package protocomm_test

import (
	"fmt"
	"math"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/boeing666/protocomm/go"
	"github.com/boeing666/protocomm/go/internal/testpb"
)

// ---------- global test server ----------

var (
	testServer *protocomm.Server
	testPort   uint16
)

func TestMain(m *testing.M) {
	greeter := &greeterService{}
	calculator := &calculatorService{}
	chat := &chatService{}

	srv, err := protocomm.NewServerBuilder().
		AddListeningPort("127.0.0.1", 0).
		SetHandshakeHeader("pc1").
		RegisterService(greeter).
		RegisterService(calculator).
		RegisterService(chat).
		BuildAndStart()
	if err != nil {
		fmt.Fprintf(os.Stderr, "server start failed: %v\n", err)
		os.Exit(1)
	}
	testServer = srv
	testPort = uint16(srv.Port())

	time.Sleep(50 * time.Millisecond)
	code := m.Run()
	testServer.Shutdown()
	os.Exit(code)
}

func makeChannel() *protocomm.Channel {
	return protocomm.CreateChannel("127.0.0.1", testPort)
}

// =========================================================
// Service implementations
// =========================================================

// --- Greeter ---

type greeterService struct{}

func (s *greeterService) RegisterWith(srv *protocomm.Server) {
	srv.RegisterMethod(protocomm.MethodID("", "Greeter", "SayHello"), s.sayHello)
	srv.RegisterMethod(protocomm.MethodID("", "Greeter", "SayHello1"), s.sayHello1)
}

func (s *greeterService) sayHello(_ *protocomm.ServerContext, req []byte) ([]byte, protocomm.Status) {
	var r testpb.HelloRequest
	if err := r.Unmarshal(req); err != nil {
		return nil, protocomm.Status{Code: protocomm.Internal, Message: err.Error()}
	}
	reply := testpb.HelloReply{Message: "Hello " + r.Name}
	return reply.Marshal(), protocomm.StatusOK()
}

func (s *greeterService) sayHello1(_ *protocomm.ServerContext, _ []byte) ([]byte, protocomm.Status) {
	return nil, protocomm.Status{Code: protocomm.InvalidArgument, Message: "bad request"}
}

// --- Calculator ---

type calculatorService struct{}

func (s *calculatorService) RegisterWith(srv *protocomm.Server) {
	srv.RegisterMethod(protocomm.MethodID("math", "Calculator", "Add"), s.add)
	srv.RegisterMethod(protocomm.MethodID("math", "Calculator", "Subtract"), s.subtract)
	srv.RegisterMethod(protocomm.MethodID("math", "Calculator", "Multiply"), s.multiply)
	srv.RegisterMethod(protocomm.MethodID("math", "Calculator", "Divide"), s.divide)
}

func calcHandler(req []byte, fn func(a, b float64) (float64, protocomm.Status)) ([]byte, protocomm.Status) {
	var r testpb.CalcRequest
	if err := r.Unmarshal(req); err != nil {
		return nil, protocomm.Status{Code: protocomm.Internal, Message: err.Error()}
	}
	result, st := fn(r.A, r.B)
	if !st.IsOK() {
		return nil, st
	}
	resp := testpb.CalcResponse{Result: result}
	return resp.Marshal(), protocomm.StatusOK()
}

func (s *calculatorService) add(_ *protocomm.ServerContext, req []byte) ([]byte, protocomm.Status) {
	return calcHandler(req, func(a, b float64) (float64, protocomm.Status) {
		return a + b, protocomm.StatusOK()
	})
}

func (s *calculatorService) subtract(_ *protocomm.ServerContext, req []byte) ([]byte, protocomm.Status) {
	return calcHandler(req, func(a, b float64) (float64, protocomm.Status) {
		return a - b, protocomm.StatusOK()
	})
}

func (s *calculatorService) multiply(_ *protocomm.ServerContext, req []byte) ([]byte, protocomm.Status) {
	return calcHandler(req, func(a, b float64) (float64, protocomm.Status) {
		return a * b, protocomm.StatusOK()
	})
}

func (s *calculatorService) divide(_ *protocomm.ServerContext, req []byte) ([]byte, protocomm.Status) {
	return calcHandler(req, func(a, b float64) (float64, protocomm.Status) {
		if b == 0 {
			return 0, protocomm.Status{Code: protocomm.InvalidArgument, Message: "division by zero"}
		}
		return a / b, protocomm.StatusOK()
	})
}

// --- Chat ---

type chatService struct {
	mu      sync.Mutex
	history []testpb.ChatMessage
}

func (s *chatService) RegisterWith(srv *protocomm.Server) {
	srv.RegisterMethod(protocomm.MethodID("messaging", "Chat", "SendMessage"), s.sendMessage)
	srv.RegisterMethod(protocomm.MethodID("messaging", "Chat", "GetHistory"), s.getHistory)
	srv.RegisterMethod(protocomm.MethodID("messaging", "Chat", "Ping"), s.ping)
}

func (s *chatService) sendMessage(_ *protocomm.ServerContext, req []byte) ([]byte, protocomm.Status) {
	var msg testpb.ChatMessage
	if err := msg.Unmarshal(req); err != nil {
		return nil, protocomm.Status{Code: protocomm.Internal, Message: err.Error()}
	}
	s.mu.Lock()
	s.history = append(s.history, msg)
	id := fmt.Sprintf("msg_%d", len(s.history))
	s.mu.Unlock()

	resp := testpb.SendResult{Success: true, MessageID: id}
	return resp.Marshal(), protocomm.StatusOK()
}

func (s *chatService) getHistory(_ *protocomm.ServerContext, req []byte) ([]byte, protocomm.Status) {
	var hr testpb.HistoryRequest
	if err := hr.Unmarshal(req); err != nil {
		return nil, protocomm.Status{Code: protocomm.Internal, Message: err.Error()}
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	limit := int(hr.Limit)
	if limit <= 0 || limit > len(s.history) {
		limit = len(s.history)
	}
	msgs := make([]testpb.ChatMessage, limit)
	copy(msgs, s.history[:limit])

	resp := testpb.HistoryResponse{Messages: msgs, Total: uint32(len(s.history))}
	return resp.Marshal(), protocomm.StatusOK()
}

func (s *chatService) ping(_ *protocomm.ServerContext, req []byte) ([]byte, protocomm.Status) {
	var pr testpb.PingRequest
	if err := pr.Unmarshal(req); err != nil {
		return nil, protocomm.Status{Code: protocomm.Internal, Message: err.Error()}
	}
	resp := testpb.PongResponse{SentAt: pr.SentAt, ReceivedAt: uint64(time.Now().UnixMilli())}
	return resp.Marshal(), protocomm.StatusOK()
}

// =========================================================
// Greeter tests
// =========================================================

func TestGreeterBasic(t *testing.T) {
	ch := makeChannel()
	defer ch.Close()

	req := testpb.HelloRequest{Name: "protocomm"}
	resp, st := ch.UnaryCall(protocomm.MethodID("", "Greeter", "SayHello"), req.Marshal())
	if !st.IsOK() {
		t.Fatalf("SayHello: %v", st)
	}
	var reply testpb.HelloReply
	if err := reply.Unmarshal(resp); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if reply.Message != "Hello protocomm" {
		t.Errorf("got %q, want %q", reply.Message, "Hello protocomm")
	}
}

func TestGreeterSequential(t *testing.T) {
	ch := makeChannel()
	defer ch.Close()

	for i := 0; i < 5; i++ {
		req := testpb.HelloRequest{Name: fmt.Sprintf("seq_%d", i)}
		resp, st := ch.UnaryCall(protocomm.MethodID("", "Greeter", "SayHello"), req.Marshal())
		if !st.IsOK() {
			t.Fatalf("call %d: %v", i, st)
		}
		var reply testpb.HelloReply
		reply.Unmarshal(resp)
		want := fmt.Sprintf("Hello seq_%d", i)
		if reply.Message != want {
			t.Errorf("call %d: got %q, want %q", i, reply.Message, want)
		}
	}
}

func TestGreeterError(t *testing.T) {
	ch := makeChannel()
	defer ch.Close()

	req := testpb.HelloRequest{Name: "err"}
	_, st := ch.UnaryCall(protocomm.MethodID("", "Greeter", "SayHello1"), req.Marshal())
	if st.Code != protocomm.InvalidArgument {
		t.Errorf("got %v, want INVALID_ARGUMENT", st)
	}
}

func TestGreeterUnimplemented(t *testing.T) {
	ch := makeChannel()
	defer ch.Close()

	req := testpb.HelloRequest{}
	_, st := ch.UnaryCall(protocomm.MethodID("", "Greeter", "SayHello2"), req.Marshal())
	if st.Code != protocomm.Unimplemented {
		t.Errorf("got %v, want UNIMPLEMENTED", st)
	}
}

func TestGreeterLargePayload(t *testing.T) {
	ch := makeChannel()
	defer ch.Close()

	name := strings.Repeat("X", 50000)
	req := testpb.HelloRequest{Name: name}
	resp, st := ch.UnaryCall(protocomm.MethodID("", "Greeter", "SayHello"), req.Marshal())
	if !st.IsOK() {
		t.Fatalf("failed: %v", st)
	}
	var reply testpb.HelloReply
	reply.Unmarshal(resp)
	if len(reply.Message) <= 50000 {
		t.Errorf("response too short: %d bytes", len(reply.Message))
	}
}

// =========================================================
// Calculator tests
// =========================================================

func TestCalculatorAdd(t *testing.T) {
	ch := makeChannel()
	defer ch.Close()

	req := testpb.CalcRequest{A: 3.14, B: 2.86}
	resp, st := ch.UnaryCall(protocomm.MethodID("math", "Calculator", "Add"), req.Marshal())
	if !st.IsOK() {
		t.Fatalf("Add: %v", st)
	}
	var r testpb.CalcResponse
	r.Unmarshal(resp)
	if math.Abs(r.Result-6.0) > 0.001 {
		t.Errorf("got %f, want 6.0", r.Result)
	}
}

func TestCalculatorSubtract(t *testing.T) {
	ch := makeChannel()
	defer ch.Close()

	req := testpb.CalcRequest{A: 10.0, B: 3.5}
	resp, st := ch.UnaryCall(protocomm.MethodID("math", "Calculator", "Subtract"), req.Marshal())
	if !st.IsOK() {
		t.Fatalf("Subtract: %v", st)
	}
	var r testpb.CalcResponse
	r.Unmarshal(resp)
	if math.Abs(r.Result-6.5) > 0.001 {
		t.Errorf("got %f, want 6.5", r.Result)
	}
}

func TestCalculatorMultiply(t *testing.T) {
	ch := makeChannel()
	defer ch.Close()

	req := testpb.CalcRequest{A: 7.0, B: 6.0}
	resp, st := ch.UnaryCall(protocomm.MethodID("math", "Calculator", "Multiply"), req.Marshal())
	if !st.IsOK() {
		t.Fatalf("Multiply: %v", st)
	}
	var r testpb.CalcResponse
	r.Unmarshal(resp)
	if math.Abs(r.Result-42.0) > 0.001 {
		t.Errorf("got %f, want 42.0", r.Result)
	}
}

func TestCalculatorDivide(t *testing.T) {
	ch := makeChannel()
	defer ch.Close()

	req := testpb.CalcRequest{A: 22.0, B: 7.0}
	resp, st := ch.UnaryCall(protocomm.MethodID("math", "Calculator", "Divide"), req.Marshal())
	if !st.IsOK() {
		t.Fatalf("Divide: %v", st)
	}
	var r testpb.CalcResponse
	r.Unmarshal(resp)
	if math.Abs(r.Result-22.0/7.0) > 0.001 {
		t.Errorf("got %f, want %f", r.Result, 22.0/7.0)
	}
}

func TestCalculatorDivideByZero(t *testing.T) {
	ch := makeChannel()
	defer ch.Close()

	req := testpb.CalcRequest{A: 1.0}
	_, st := ch.UnaryCall(protocomm.MethodID("math", "Calculator", "Divide"), req.Marshal())
	if st.Code != protocomm.InvalidArgument {
		t.Errorf("got %v, want INVALID_ARGUMENT", st)
	}
}

// =========================================================
// Chat tests
// =========================================================

func TestChatSendAndHistory(t *testing.T) {
	ch := makeChannel()
	defer ch.Close()

	for i := 0; i < 3; i++ {
		msg := testpb.ChatMessage{
			Sender:    fmt.Sprintf("user%d", i),
			Text:      fmt.Sprintf("Message #%d", i),
			Timestamp: uint64(1000 + i),
		}
		resp, st := ch.UnaryCall(protocomm.MethodID("messaging", "Chat", "SendMessage"), msg.Marshal())
		if !st.IsOK() {
			t.Fatalf("SendMessage %d: %v", i, st)
		}
		var result testpb.SendResult
		result.Unmarshal(resp)
		if !result.Success {
			t.Errorf("SendMessage %d: success=false", i)
		}
		if result.MessageID == "" {
			t.Errorf("SendMessage %d: empty message_id", i)
		}
	}

	histReq := testpb.HistoryRequest{Limit: 10}
	resp, st := ch.UnaryCall(protocomm.MethodID("messaging", "Chat", "GetHistory"), histReq.Marshal())
	if !st.IsOK() {
		t.Fatalf("GetHistory: %v", st)
	}
	var hist testpb.HistoryResponse
	hist.Unmarshal(resp)
	if hist.Total != 3 {
		t.Errorf("total: got %d, want 3", hist.Total)
	}
	if len(hist.Messages) != 3 {
		t.Fatalf("messages: got %d, want 3", len(hist.Messages))
	}
	if hist.Messages[0].Sender != "user0" {
		t.Errorf("first sender: got %q", hist.Messages[0].Sender)
	}
	if hist.Messages[2].Text != "Message #2" {
		t.Errorf("last text: got %q", hist.Messages[2].Text)
	}
}

func TestChatPing(t *testing.T) {
	ch := makeChannel()
	defer ch.Close()

	now := uint64(time.Now().UnixMilli())
	req := testpb.PingRequest{SentAt: now}
	resp, st := ch.UnaryCall(protocomm.MethodID("messaging", "Chat", "Ping"), req.Marshal())
	if !st.IsOK() {
		t.Fatalf("Ping: %v", st)
	}
	var pong testpb.PongResponse
	pong.Unmarshal(resp)
	if pong.SentAt != now {
		t.Errorf("sent_at: got %d, want %d", pong.SentAt, now)
	}
	if pong.ReceivedAt == 0 {
		t.Error("received_at is 0")
	}
	if pong.ReceivedAt < pong.SentAt {
		t.Errorf("received_at (%d) < sent_at (%d)", pong.ReceivedAt, pong.SentAt)
	}
}

// =========================================================
// Cross-service & misc tests
// =========================================================

func TestMultipleServicesOnSameServer(t *testing.T) {
	ch := makeChannel()
	defer ch.Close()

	// Greeter
	greq := testpb.HelloRequest{Name: "multi"}
	resp, st := ch.UnaryCall(protocomm.MethodID("", "Greeter", "SayHello"), greq.Marshal())
	if !st.IsOK() {
		t.Fatalf("Greeter: %v", st)
	}
	var gr testpb.HelloReply
	gr.Unmarshal(resp)
	if gr.Message != "Hello multi" {
		t.Errorf("greeter: got %q", gr.Message)
	}

	// Calculator
	creq := testpb.CalcRequest{A: 2.0, B: 3.0}
	resp, st = ch.UnaryCall(protocomm.MethodID("math", "Calculator", "Add"), creq.Marshal())
	if !st.IsOK() {
		t.Fatalf("Calculator: %v", st)
	}
	var cr testpb.CalcResponse
	cr.Unmarshal(resp)
	if math.Abs(cr.Result-5.0) > 0.001 {
		t.Errorf("calc: got %f, want 5.0", cr.Result)
	}

	// Chat
	preq := testpb.PingRequest{SentAt: 12345}
	resp, st = ch.UnaryCall(protocomm.MethodID("messaging", "Chat", "Ping"), preq.Marshal())
	if !st.IsOK() {
		t.Fatalf("Chat: %v", st)
	}
}

func TestHandshakeMismatch(t *testing.T) {
	ch := protocomm.CreateChannel("127.0.0.1", testPort, protocomm.ChannelConfig{
		HandshakeHeader: "BAD",
	})
	defer ch.Close()

	req := testpb.HelloRequest{}
	_, st := ch.UnaryCall(protocomm.MethodID("", "Greeter", "SayHello"), req.Marshal())
	if st.IsOK() {
		t.Error("expected error for mismatched handshake")
	}
}

func TestConcurrentCalls(t *testing.T) {
	const n = 10
	var wg sync.WaitGroup
	errs := make(chan error, n)

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			ch := makeChannel()
			defer ch.Close()

			req := testpb.HelloRequest{Name: fmt.Sprintf("c_%d", idx)}
			resp, st := ch.UnaryCall(protocomm.MethodID("", "Greeter", "SayHello"), req.Marshal())
			if !st.IsOK() {
				errs <- fmt.Errorf("goroutine %d: %v", idx, st)
				return
			}
			var reply testpb.HelloReply
			reply.Unmarshal(resp)
			want := fmt.Sprintf("Hello c_%d", idx)
			if reply.Message != want {
				errs <- fmt.Errorf("goroutine %d: got %q, want %q", idx, reply.Message, want)
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}

func TestServerShutdown(t *testing.T) {
	svc := &greeterService{}
	srv, err := protocomm.NewServerBuilder().
		AddListeningPort("127.0.0.1", 0).
		SetHandshakeHeader("pc1").
		RegisterService(svc).
		BuildAndStart()
	if err != nil {
		t.Fatal(err)
	}
	port := uint16(srv.Port())

	ch := protocomm.CreateChannel("127.0.0.1", port)
	req := testpb.HelloRequest{Name: "shutdown"}
	resp, st := ch.UnaryCall(protocomm.MethodID("", "Greeter", "SayHello"), req.Marshal())
	if !st.IsOK() {
		t.Fatalf("before shutdown: %v", st)
	}
	var reply testpb.HelloReply
	reply.Unmarshal(resp)
	if reply.Message != "Hello shutdown" {
		t.Errorf("got %q", reply.Message)
	}
	ch.Close()

	srv.Shutdown()

	ch2 := protocomm.CreateChannel("127.0.0.1", port)
	defer ch2.Close()
	_, st = ch2.UnaryCall(protocomm.MethodID("", "Greeter", "SayHello"), req.Marshal())
	if st.IsOK() {
		t.Error("expected error after shutdown")
	}
}

func TestEmptyRequest(t *testing.T) {
	ch := makeChannel()
	defer ch.Close()

	req := testpb.HelloRequest{}
	resp, st := ch.UnaryCall(protocomm.MethodID("", "Greeter", "SayHello"), req.Marshal())
	if !st.IsOK() {
		t.Fatalf("empty request: %v", st)
	}
	var reply testpb.HelloReply
	reply.Unmarshal(resp)
	if reply.Message != "Hello " {
		t.Errorf("got %q, want %q", reply.Message, "Hello ")
	}
}

func TestMultipleCallsSameChannel(t *testing.T) {
	ch := makeChannel()
	defer ch.Close()

	for i := 0; i < 20; i++ {
		req := testpb.CalcRequest{A: float64(i), B: 1.0}
		resp, st := ch.UnaryCall(protocomm.MethodID("math", "Calculator", "Add"), req.Marshal())
		if !st.IsOK() {
			t.Fatalf("call %d: %v", i, st)
		}
		var r testpb.CalcResponse
		r.Unmarshal(resp)
		if math.Abs(r.Result-float64(i+1)) > 0.001 {
			t.Errorf("call %d: got %f, want %f", i, r.Result, float64(i+1))
		}
	}
}
