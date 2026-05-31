package protocomm_test

import (
	"math"
	"os"
	"strconv"
	"testing"

	"protocomm"
	"protocomm/internal/testpb"
)

// =========================================================
// Wire-level compatibility tests
// =========================================================

func TestWireHashCompatibility(t *testing.T) {
	// The C++ protoc plugin generates method IDs using:
	//   Fnv1a32("/[package.]Service/Method")
	// Verify the Go implementation produces the same format.
	tests := []struct {
		pkg, svc, method, fullName string
	}{
		{"", "Greeter", "SayHello", "/Greeter/SayHello"},
		{"", "Greeter", "SayHello1", "/Greeter/SayHello1"},
		{"", "Greeter", "SayHello2", "/Greeter/SayHello2"},
		{"math", "Calculator", "Add", "/math.Calculator/Add"},
		{"math", "Calculator", "Subtract", "/math.Calculator/Subtract"},
		{"math", "Calculator", "Multiply", "/math.Calculator/Multiply"},
		{"math", "Calculator", "Divide", "/math.Calculator/Divide"},
		{"messaging", "Chat", "SendMessage", "/messaging.Chat/SendMessage"},
		{"messaging", "Chat", "GetHistory", "/messaging.Chat/GetHistory"},
		{"messaging", "Chat", "Ping", "/messaging.Chat/Ping"},
	}
	for _, tt := range tests {
		id := protocomm.MethodID(tt.pkg, tt.svc, tt.method)
		expected := protocomm.Fnv1a32(tt.fullName)
		if id != expected {
			t.Errorf("MethodID(%q,%q,%q) = 0x%08x, Fnv1a32(%q) = 0x%08x",
				tt.pkg, tt.svc, tt.method, id, tt.fullName, expected)
		}
		t.Logf("%-40s → 0x%08x", tt.fullName, id)
	}
}

func TestWireFrameFormat(t *testing.T) {
	// Verify the 16-byte frame layout matches the C++ FrameHeader::Serialize.
	hdr := protocomm.FrameHeader{
		Type:        protocomm.FrameRequest,
		StatusCode:  0,
		CallID:      1,
		MethodID:    protocomm.MethodID("", "Greeter", "SayHello"),
		PayloadSize: 13,
	}
	buf := hdr.Serialize()
	if len(buf) != 16 {
		t.Fatalf("size: got %d, want 16", len(buf))
	}
	// type byte
	if buf[0] != 1 {
		t.Errorf("type: got %d, want 1", buf[0])
	}
	// flags always 0
	if buf[1] != 0 {
		t.Errorf("flags: got %d, want 0", buf[1])
	}
	// round-trip
	decoded := protocomm.DeserializeFrameHeader(buf)
	if decoded != hdr {
		t.Errorf("round-trip:\n got %+v\nwant %+v", decoded, hdr)
	}
}

func TestWireHandshakeBytes(t *testing.T) {
	// Default handshake header "pc1" = {0x70, 0x63, 0x31}.
	expected := []byte{0x70, 0x63, 0x31}
	actual := []byte("pc1")
	if len(actual) != len(expected) {
		t.Fatalf("len: got %d, want %d", len(actual), len(expected))
	}
	for i := range expected {
		if actual[i] != expected[i] {
			t.Errorf("byte %d: got 0x%02x, want 0x%02x", i, actual[i], expected[i])
		}
	}
}

func TestProtobufEncoding(t *testing.T) {
	t.Run("HelloRequest", func(t *testing.T) {
		req := testpb.HelloRequest{Name: "test"}
		data := req.Marshal()
		// tag(1, LEN) = 0x0A, length = 4, then "test"
		expected := []byte{0x0A, 0x04, 't', 'e', 's', 't'}
		if len(data) != len(expected) {
			t.Fatalf("len: got %d, want %d\n data: %v\n want: %v", len(data), len(expected), data, expected)
		}
		for i := range expected {
			if data[i] != expected[i] {
				t.Errorf("byte %d: got 0x%02x, want 0x%02x", i, data[i], expected[i])
			}
		}
	})

	t.Run("HelloRequest_empty", func(t *testing.T) {
		req := testpb.HelloRequest{}
		data := req.Marshal()
		if len(data) != 0 {
			t.Errorf("empty message should be 0 bytes, got %d: %v", len(data), data)
		}
	})

	t.Run("CalcRequest_roundtrip", func(t *testing.T) {
		req := testpb.CalcRequest{A: 1.5, B: 2.5}
		data := req.Marshal()
		var decoded testpb.CalcRequest
		if err := decoded.Unmarshal(data); err != nil {
			t.Fatal(err)
		}
		if decoded.A != 1.5 || decoded.B != 2.5 {
			t.Errorf("got A=%f B=%f", decoded.A, decoded.B)
		}
	})

	t.Run("CalcRequest_binary", func(t *testing.T) {
		req := testpb.CalcRequest{A: 1.0, B: 2.0}
		data := req.Marshal()
		// field 1 (double): tag = 0x09 (field 1, wire type 1=fixed64), 8 bytes LE for 1.0
		// field 2 (double): tag = 0x11 (field 2, wire type 1=fixed64), 8 bytes LE for 2.0
		if len(data) != 18 {
			t.Fatalf("CalcRequest(1.0, 2.0) should be 18 bytes, got %d: %v", len(data), data)
		}
		if data[0] != 0x09 {
			t.Errorf("field 1 tag: got 0x%02x, want 0x09", data[0])
		}
		if data[9] != 0x11 {
			t.Errorf("field 2 tag: got 0x%02x, want 0x11", data[9])
		}
	})

	t.Run("ChatMessage_roundtrip", func(t *testing.T) {
		msg := testpb.ChatMessage{Sender: "alice", Text: "hi", Timestamp: 42}
		data := msg.Marshal()
		var decoded testpb.ChatMessage
		if err := decoded.Unmarshal(data); err != nil {
			t.Fatal(err)
		}
		if decoded.Sender != "alice" || decoded.Text != "hi" || decoded.Timestamp != 42 {
			t.Errorf("got %+v", decoded)
		}
	})

	t.Run("HistoryResponse_roundtrip", func(t *testing.T) {
		resp := testpb.HistoryResponse{
			Messages: []testpb.ChatMessage{
				{Sender: "a", Text: "hello"},
				{Sender: "b", Text: "world"},
			},
			Total: 2,
		}
		data := resp.Marshal()
		var decoded testpb.HistoryResponse
		if err := decoded.Unmarshal(data); err != nil {
			t.Fatal(err)
		}
		if len(decoded.Messages) != 2 || decoded.Total != 2 {
			t.Fatalf("got %+v", decoded)
		}
		if decoded.Messages[0].Sender != "a" || decoded.Messages[1].Text != "world" {
			t.Errorf("messages: %+v", decoded.Messages)
		}
	})

	t.Run("PongResponse_roundtrip", func(t *testing.T) {
		resp := testpb.PongResponse{SentAt: 1000, ReceivedAt: 2000}
		data := resp.Marshal()
		var decoded testpb.PongResponse
		if err := decoded.Unmarshal(data); err != nil {
			t.Fatal(err)
		}
		if decoded.SentAt != 1000 || decoded.ReceivedAt != 2000 {
			t.Errorf("got %+v", decoded)
		}
	})
}

// =========================================================
// Cross-language interop: Go client → C++ server
// =========================================================

// TestGoClientToCppServer connects to a running C++ server.
//
// Usage:
//
//	# Terminal 1: start C++ test server on port 51234
//	./build/protocomm_tests
//
//	# Terminal 2: run Go interop tests
//	CPP_PORT=51234 go test -run TestGoClientToCppServer -v
func TestGoClientToCppServer(t *testing.T) {
	portStr := os.Getenv("CPP_PORT")
	if portStr == "" {
		t.Skip("CPP_PORT not set; start C++ server and set CPP_PORT to enable")
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("invalid CPP_PORT: %v", err)
	}
	ch := protocomm.CreateChannel("127.0.0.1", uint16(port))
	defer ch.Close()

	t.Run("Greeter_SayHello", func(t *testing.T) {
		req := testpb.HelloRequest{Name: "Go client"}
		resp, st := ch.UnaryCall(protocomm.MethodID("", "Greeter", "SayHello"), req.Marshal())
		if !st.IsOK() {
			t.Fatalf("status: %v", st)
		}
		var reply testpb.HelloReply
		reply.Unmarshal(resp)
		if reply.Message != "Hello Go client" {
			t.Errorf("got %q, want %q", reply.Message, "Hello Go client")
		}
	})

	t.Run("Greeter_Error", func(t *testing.T) {
		req := testpb.HelloRequest{Name: "err"}
		_, st := ch.UnaryCall(protocomm.MethodID("", "Greeter", "SayHello1"), req.Marshal())
		if st.Code != protocomm.InvalidArgument {
			t.Errorf("got %v, want INVALID_ARGUMENT", st)
		}
	})

	t.Run("Greeter_Unimplemented", func(t *testing.T) {
		req := testpb.HelloRequest{}
		_, st := ch.UnaryCall(protocomm.MethodID("", "Greeter", "SayHello2"), req.Marshal())
		if st.Code != protocomm.Unimplemented {
			t.Errorf("got %v, want UNIMPLEMENTED", st)
		}
	})

	t.Run("Calculator_Add", func(t *testing.T) {
		req := testpb.CalcRequest{A: 3.14, B: 2.86}
		resp, st := ch.UnaryCall(protocomm.MethodID("math", "Calculator", "Add"), req.Marshal())
		if !st.IsOK() {
			t.Fatalf("status: %v", st)
		}
		var r testpb.CalcResponse
		r.Unmarshal(resp)
		if math.Abs(r.Result-6.0) > 0.001 {
			t.Errorf("got %f, want 6.0", r.Result)
		}
	})

	t.Run("Calculator_Subtract", func(t *testing.T) {
		req := testpb.CalcRequest{A: 10.0, B: 3.5}
		resp, st := ch.UnaryCall(protocomm.MethodID("math", "Calculator", "Subtract"), req.Marshal())
		if !st.IsOK() {
			t.Fatalf("status: %v", st)
		}
		var r testpb.CalcResponse
		r.Unmarshal(resp)
		if math.Abs(r.Result-6.5) > 0.001 {
			t.Errorf("got %f, want 6.5", r.Result)
		}
	})

	t.Run("Calculator_Multiply", func(t *testing.T) {
		req := testpb.CalcRequest{A: 7.0, B: 6.0}
		resp, st := ch.UnaryCall(protocomm.MethodID("math", "Calculator", "Multiply"), req.Marshal())
		if !st.IsOK() {
			t.Fatalf("status: %v", st)
		}
		var r testpb.CalcResponse
		r.Unmarshal(resp)
		if math.Abs(r.Result-42.0) > 0.001 {
			t.Errorf("got %f, want 42.0", r.Result)
		}
	})

	t.Run("Calculator_Divide", func(t *testing.T) {
		req := testpb.CalcRequest{A: 22.0, B: 7.0}
		resp, st := ch.UnaryCall(protocomm.MethodID("math", "Calculator", "Divide"), req.Marshal())
		if !st.IsOK() {
			t.Fatalf("status: %v", st)
		}
		var r testpb.CalcResponse
		r.Unmarshal(resp)
		if math.Abs(r.Result-22.0/7.0) > 0.001 {
			t.Errorf("got %f, want %f", r.Result, 22.0/7.0)
		}
	})

	t.Run("Calculator_DivideByZero", func(t *testing.T) {
		req := testpb.CalcRequest{A: 1.0}
		_, st := ch.UnaryCall(protocomm.MethodID("math", "Calculator", "Divide"), req.Marshal())
		if st.Code != protocomm.InvalidArgument {
			t.Errorf("got %v, want INVALID_ARGUMENT", st)
		}
	})

	t.Run("Chat_Ping", func(t *testing.T) {
		req := testpb.PingRequest{SentAt: 12345}
		resp, st := ch.UnaryCall(protocomm.MethodID("messaging", "Chat", "Ping"), req.Marshal())
		if !st.IsOK() {
			t.Fatalf("status: %v", st)
		}
		var pong testpb.PongResponse
		pong.Unmarshal(resp)
		if pong.SentAt != 12345 {
			t.Errorf("sent_at: got %d, want 12345", pong.SentAt)
		}
		if pong.ReceivedAt == 0 {
			t.Error("received_at is 0")
		}
	})
}

// =========================================================
// Cross-language interop: Go server for C++ client
// =========================================================

// TestGoServerForCppClient starts a Go server that C++ tests can connect to.
//
// Usage:
//
//	# Terminal 1: start Go server
//	GO_SERVER=1 GO_SERVER_PORT=51235 go test -run TestGoServerForCppClient -timeout 60s -v
//
//	# Terminal 2: modify C++ test to use port 51235 and run it
func TestGoServerForCppClient(t *testing.T) {
	if os.Getenv("GO_SERVER") == "" {
		t.Skip("GO_SERVER not set; set GO_SERVER=1 to start a server for C++ client testing")
	}

	portStr := os.Getenv("GO_SERVER_PORT")
	if portStr == "" {
		portStr = "51235"
	}
	port, _ := strconv.Atoi(portStr)

	srv, err := protocomm.NewServerBuilder().
		AddListeningPort("127.0.0.1", uint16(port)).
		SetHandshakeHeader("pc1").
		RegisterService(&greeterService{}).
		RegisterService(&calculatorService{}).
		RegisterService(&chatService{}).
		BuildAndStart()
	if err != nil {
		t.Fatalf("BuildAndStart: %v", err)
	}
	defer srv.Shutdown()

	t.Logf("Go server listening on port %d — waiting for C++ client...", srv.Port())
	t.Log("Press Ctrl+C or wait for timeout to stop.")
	srv.Wait()
}
