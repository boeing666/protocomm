package main

import (
	"log"
	"time"

	protocomm "github.com/boeing666/protocomm/go"
	"github.com/boeing666/protocomm/go/proto/example"
)

type greeter struct {
	example.UnimplementedGreeterServer
}

func (greeter) SayHello(_ *protocomm.ServerContext, req *example.HelloRequest) (*example.HelloReply, protocomm.Status) {
	return &example.HelloReply{Message: "Hello " + req.Name}, protocomm.StatusOK()
}

func traceInterceptor(ctx *protocomm.ServerContext, code protocomm.StatusCode, req, resp []byte, dur time.Duration) {
	log.Printf("[trace] %-16s peer=%s code=%s %v bytes=%d/%d\n        req  = %s\n        resp = %s",
		ctx.MethodName, ctx.PeerAddress, code, dur.Round(time.Microsecond),
		len(req), len(resp), ctx.RequestText, ctx.ResponseText)
}

func main() {
	log.SetFlags(0)

	srv, err := protocomm.NewServerBuilder().
		AddListeningPort("127.0.0.1", 0).
		SetHandshakeHeader("pc1").
		RegisterService(example.NewGreeterService(greeter{})).
		SetInterceptor(traceInterceptor).
		BuildAndStart()
	if err != nil {
		log.Fatalf("server start: %v", err)
	}
	defer srv.Shutdown()

	port := uint16(srv.Port())
	log.Printf("server listening on 127.0.0.1:%d\n", port)

	ch := protocomm.CreateChannel("127.0.0.1", port)
	defer ch.Close()

	client := example.NewGreeterClient(ch)

	for _, name := range []string{"world", "protocomm", "trace"} {
		resp, st := client.SayHello(&example.HelloRequest{Name: name})
		if !st.IsOK() {
			log.Printf("call failed: %v", st)
			continue
		}
		log.Printf("reply: %q\n", resp.Message)
	}

	if _, st := client.SayHello2(&example.HelloRequest{Name: "nobody"}); !st.IsOK() {
		log.Printf("expected error: %v\n", st)
	}

	time.Sleep(100 * time.Millisecond)
}
