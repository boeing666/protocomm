package protocomm_test

import (
	"strings"
	"sync"
	"testing"
	"time"

	protocomm "github.com/boeing666/protocomm/go"
	mathpb "github.com/boeing666/protocomm/go/proto/math"
)

type calcImpl struct {
	mathpb.UnimplementedCalculatorServer
}

func (calcImpl) Add(_ *protocomm.ServerContext, req *mathpb.CalcRequest) (*mathpb.CalcResponse, protocomm.Status) {
	return &mathpb.CalcResponse{Result: req.A + req.B}, protocomm.StatusOK()
}

func TestInterceptorTrace(t *testing.T) {
	var (
		mu                       sync.Mutex
		gotName, gotReq, gotResp string
		gotOK                    bool
	)

	srv, err := protocomm.NewServerBuilder().
		AddListeningPort("127.0.0.1", 0).
		SetHandshakeHeader("pc1").
		RegisterService(mathpb.NewCalculatorService(calcImpl{})).
		SetInterceptor(func(ctx *protocomm.ServerContext, code protocomm.StatusCode, _, _ []byte, _ time.Duration) {
			mu.Lock()
			gotName = ctx.MethodName
			gotReq = ctx.RequestText
			gotResp = ctx.ResponseText
			gotOK = code == protocomm.OK
			mu.Unlock()
		}).
		BuildAndStart()
	if err != nil {
		t.Fatalf("server start: %v", err)
	}
	defer srv.Shutdown()
	time.Sleep(50 * time.Millisecond)

	ch := protocomm.CreateChannel("127.0.0.1", uint16(srv.Port()))
	defer ch.Close()

	client := mathpb.NewCalculatorClient(ch)
	resp, st := client.Add(&mathpb.CalcRequest{A: 2, B: 3})
	if !st.IsOK() {
		t.Fatalf("Add: %v", st)
	}
	if resp.Result != 5 {
		t.Fatalf("result: got %v, want 5", resp.Result)
	}

	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if !gotOK {
		t.Error("interceptor: status not OK")
	}
	if gotName != "Calculator.Add" {
		t.Errorf("method name: got %q, want %q", gotName, "Calculator.Add")
	}
	if !strings.Contains(gotReq, "2") || !strings.Contains(gotReq, "3") {
		t.Errorf("request_text not decoded: %q", gotReq)
	}
	if !strings.Contains(gotResp, "5") {
		t.Errorf("response_text not decoded: %q", gotResp)
	}
}
