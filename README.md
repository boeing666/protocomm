# protocomm

Lightweight RPC library for C++20 and Go. Unary calls + server push over plain TCP, with protobuf payloads and a custom 16-byte frame. Methods are routed by 32-bit FNV-1a hash of `/package.Service/Method`, computed at compile time.

C++ implementation uses Boost.Asio with a single io thread per channel and a coroutine-based reader; in-flight calls are multiplexed by `call_id`, so multiple concurrent requests share one TCP connection.

## When to use it

- You want a gRPC-shaped API (`Stub` / `Service` / `ServerBuilder`) without HTTP/2, TLS, or gRPC's runtime weight
- You need server-initiated push frames on the same connection as RPC
- You control both client and server (no external interop requirements)

## Build

Requirements: CMake >= 3.21, MSVC (C++20), Go 1.21+, Boost >= 1.78, Protobuf >= v29.6. Missing deps are fetched via `FetchContent`.

```
cmake -S . -B cmake-build-debug -G Ninja
cmake --build cmake-build-debug
```

Outputs:
- `protocomm` static library (C++)
- `protoc-gen-protocomm` plugin
- `protoc-gen-protocomm-go` plugin
- Generated stubs for `proto/*.proto`

## Integrate into your project

### C++

```cmake
set(PROTOCOMM_BUILD_TESTS OFF CACHE BOOL "" FORCE)
add_subdirectory(third_party/protocomm)

add_executable(myapp main.cpp)
target_link_libraries(myapp PRIVATE protocomm)
target_include_directories(myapp PRIVATE
    third_party/protocomm/c++/include)
```

### Go

```
go get github.com/boeing666/protocomm/go
go install github.com/boeing666/protocomm/go/protoc-gen-protocomm-go@latest
protoc --go_out=. --go_opt=paths=source_relative \
       --protocomm-go_out=. --protocomm-go_opt=paths=source_relative \
       greeter.proto
```

## Define a service

`greeter.proto`:
```proto
syntax = "proto3";
option go_package = "yourmod/proto";

service Greeter {
  rpc SayHello (HelloRequest) returns (HelloReply);
}

message HelloRequest { string name = 1; }
message HelloReply   { string message = 1; }
```

Generate stubs:
```
protoc --plugin=protoc-gen-protocomm=./protoc-gen-protocomm \
       --protocomm_out=. greeter.proto
```

The plugin pre-scans all methods in the request and fails with an error if two methods hash to the same 32-bit ID.

## C++ server

```cpp
#include "protocomm/protocomm.h"
#include "protocomm/Greeter.h"

class GreeterImpl : public Greeter::Service {
    protocomm::Status SayHello(
            protocomm::ServerContext*,
            const HelloRequest* req,
            HelloReply* resp) override {
        resp->set_message("Hello " + req->name());
        return {};
    }
};

int main() {
    GreeterImpl impl;
    auto server = protocomm::ServerBuilder()
        .AddListeningPort("0.0.0.0", 50051)
        .SetHandshakeHeader("pc1")
        .SetIoThreadCount(4)
        .RegisterService(&impl)
        .BuildAndStart();
    server->Wait();
}
```

## C++ client

Three call styles, same channel:

```cpp
auto ch = protocomm::CreateChannel("127.0.0.1", 50051,
                                   {.handshake_header = "pc1"});
Greeter::Stub stub(ch);
HelloRequest req;
req.set_name("world");

// 1. Sync
HelloReply resp;
auto st = stub.SayHello(req, &resp);

// 2. Async + future
auto [st2, resp2] = stub.AsyncSayHello(req).get();

// 3. Async + callback (fire and continue)
stub.AsyncSayHello(req,
    [](protocomm::Status s, HelloReply r) {
        // runs on the channel's io thread; do not block
    });
// execution continues here immediately
```

Multiple in-flight calls on the same channel are multiplexed; no per-call thread is spawned. Callbacks fire on the channel's io thread, so push heavy work to your own pool from inside them.

## Go server and client

```
import (
    pc "github.com/boeing666/protocomm/go"
    pb "yourmod/proto"
)

type greeter struct{ pb.UnimplementedGreeterServer }

func (greeter) SayHello(_ *pc.ServerContext, req *pb.HelloRequest) (*pb.HelloReply, pc.Status) {
    return &pb.HelloReply{Message: "Hello " + req.Name}, pc.StatusOK()
}

func main() {
    srv, _ := pc.NewServerBuilder().
        AddListeningPort("0.0.0.0", 50051).
        RegisterService(pb.NewGreeterService(greeter{})).
        BuildAndStart()
    srv.Wait()
}
```

```
ch := pc.CreateChannel("127.0.0.1", 50051)
client := pb.NewGreeterClient(ch)
resp, st := client.SayHello(&pb.HelloRequest{Name: "world"})
```

## Server push

The server may push frames to any peer at any time. Generated `PeerWriter` and `Broadcaster` wrap typed messages:

```cpp
Greeter::Broadcaster(server.get())
    .BroadcastSayHello(HelloReply{...});
```

The client receives them through `ChannelConfig::on_push`:

```cpp
protocomm::CreateChannel(host, port, {
    .on_push = [](uint32_t method_id, const std::string& payload) {
        // dispatch by method_id
    },
});
```
