#include <chrono>
#include <cmath>
#include <iostream>
#include <mutex>
#include <string>
#include <thread>
#include <vector>

#include "protocomm/protocomm.h"
#include "protocomm/Greeter.h"
#include "protocomm/math/Calculator.h"
#include "protocomm/messaging/Chat.h"

static int test_count = 0;
static int pass_count = 0;

#define TEST_ASSERT(cond, msg) do { \
    test_count++; \
    if (!(cond)) { \
        std::cerr << "FAIL: " << msg << "\n"; \
        return false; \
    } \
    pass_count++; \
    std::cout << "  PASS: " << msg << "\n"; \
} while(0)

class GreeterServiceImpl : public Greeter::Service {
public:
    ::protocomm::Status SayHello(
            ::protocomm::ServerContext* ,
            const HelloRequest* req,
            HelloReply* resp) override {
        resp->set_message("Hello " + req->name());
        return {};
    }

    ::protocomm::Status SayHello1(
            ::protocomm::ServerContext* ,
            const HelloRequest* ,
            HelloReply* ) override {
        return {::protocomm::StatusCode::INVALID_ARGUMENT, "bad request"};
    }
};

class CalculatorServiceImpl : public math::Calculator::Service {
public:
    ::protocomm::Status Add(
            ::protocomm::ServerContext* ,
            const math::CalcRequest* req,
            math::CalcResponse* resp) override {
        resp->set_result(req->a() + req->b());
        return {};
    }

    ::protocomm::Status Subtract(
            ::protocomm::ServerContext* ,
            const math::CalcRequest* req,
            math::CalcResponse* resp) override {
        resp->set_result(req->a() - req->b());
        return {};
    }

    ::protocomm::Status Multiply(
            ::protocomm::ServerContext* ,
            const math::CalcRequest* req,
            math::CalcResponse* resp) override {
        resp->set_result(req->a() * req->b());
        return {};
    }

    ::protocomm::Status Divide(
            ::protocomm::ServerContext* ,
            const math::CalcRequest* req,
            math::CalcResponse* resp) override {
        if (req->b() == 0.0) {
            return {::protocomm::StatusCode::INVALID_ARGUMENT, "division by zero"};
        }
        resp->set_result(req->a() / req->b());
        return {};
    }
};

class ChatServiceImpl : public messaging::Chat::Service {
public:
    ::protocomm::Status SendMessage(
            ::protocomm::ServerContext* ,
            const messaging::ChatMessage* req,
            messaging::SendResult* resp) override {
        std::lock_guard lock(mu_);
        history_.push_back(*req);
        resp->set_success(true);
        resp->set_message_id("msg_" + std::to_string(history_.size()));
        return {};
    }

    ::protocomm::Status GetHistory(
            ::protocomm::ServerContext* ,
            const messaging::HistoryRequest* req,
            messaging::HistoryResponse* resp) override {
        std::lock_guard lock(mu_);
        uint32_t limit = req->limit() > 0 ? req->limit()
                                           : static_cast<uint32_t>(history_.size());
        uint32_t count = std::min(limit, static_cast<uint32_t>(history_.size()));
        for (uint32_t i = 0; i < count; i++) {
            *resp->add_messages() = history_[i];
        }
        resp->set_total(static_cast<uint32_t>(history_.size()));
        return {};
    }

    ::protocomm::Status Ping(
            ::protocomm::ServerContext* ,
            const messaging::PingRequest* req,
            messaging::PongResponse* resp) override {
        resp->set_sent_at(req->sent_at());
        resp->set_received_at(static_cast<uint64_t>(
            std::chrono::duration_cast<std::chrono::milliseconds>(
                std::chrono::system_clock::now().time_since_epoch()
            ).count()
        ));
        return {};
    }

private:
    std::mutex mu_;
    std::vector<messaging::ChatMessage> history_;
};

static std::shared_ptr<protocomm::Channel> make_channel(uint16_t port) {
    return protocomm::CreateChannel("127.0.0.1", port,
                                    {.handshake_header = "pc1"});
}

bool test_greeter_basic(uint16_t port) {
    auto ch = make_channel(port);
    Greeter::Stub stub(ch);

    HelloRequest req;
    req.set_name("protocomm");
    auto result = stub.AsyncSayHello(req).get();
    TEST_ASSERT(result.first.ok(), "greeter basic call");
    TEST_ASSERT(result.second.message() == "Hello protocomm", "greeter response");
    return true;
}

bool test_greeter_sync(uint16_t port) {
    auto ch = make_channel(port);
    Greeter::Stub stub(ch);

    HelloRequest req;
    req.set_name("sync_test");
    HelloReply resp;
    auto status = stub.SayHello(req, &resp);
    TEST_ASSERT(status.ok(), "greeter sync call");
    TEST_ASSERT(resp.message() == "Hello sync_test", "greeter sync response");
    return true;
}

bool test_greeter_sequential(uint16_t port) {
    auto ch = make_channel(port);
    Greeter::Stub stub(ch);

    for (int i = 0; i < 5; i++) {
        HelloRequest req;
        req.set_name("seq_" + std::to_string(i));
        auto result = stub.AsyncSayHello(req).get();
        TEST_ASSERT(result.first.ok(), "greeter seq call " + std::to_string(i));
    }
    return true;
}

bool test_greeter_error(uint16_t port) {
    auto ch = make_channel(port);
    Greeter::Stub stub(ch);

    HelloRequest req;
    req.set_name("err");
    auto result = stub.AsyncSayHello1(req).get();
    TEST_ASSERT(result.first.error_code() == protocomm::StatusCode::INVALID_ARGUMENT,
                "greeter error status");
    return true;
}

bool test_greeter_unimplemented(uint16_t port) {
    auto ch = make_channel(port);
    Greeter::Stub stub(ch);

    HelloRequest req;
    auto result = stub.AsyncSayHello2(req).get();
    TEST_ASSERT(result.first.error_code() == protocomm::StatusCode::UNIMPLEMENTED,
                "greeter unimplemented");
    return true;
}

bool test_greeter_large_payload(uint16_t port) {
    auto ch = make_channel(port);
    Greeter::Stub stub(ch);

    HelloRequest req;
    req.set_name(std::string(50000, 'X'));
    auto result = stub.AsyncSayHello(req).get();
    TEST_ASSERT(result.first.ok(), "greeter large payload");
    TEST_ASSERT(result.second.message().size() > 50000, "greeter large response size");
    return true;
}

bool test_calc_add(uint16_t port) {
    auto ch = make_channel(port);
    math::Calculator::Stub stub(ch);

    math::CalcRequest req;
    req.set_a(3.14);
    req.set_b(2.86);
    auto result = stub.AsyncAdd(req).get();
    TEST_ASSERT(result.first.ok(), "calc add call");
    TEST_ASSERT(std::abs(result.second.result() - 6.0) < 0.001, "calc add result");
    return true;
}

bool test_calc_subtract(uint16_t port) {
    auto ch = make_channel(port);
    math::Calculator::Stub stub(ch);

    math::CalcRequest req;
    req.set_a(10.0);
    req.set_b(3.5);
    auto result = stub.AsyncSubtract(req).get();
    TEST_ASSERT(result.first.ok(), "calc subtract call");
    TEST_ASSERT(std::abs(result.second.result() - 6.5) < 0.001, "calc subtract result");
    return true;
}

bool test_calc_multiply(uint16_t port) {
    auto ch = make_channel(port);
    math::Calculator::Stub stub(ch);

    math::CalcRequest req;
    req.set_a(7.0);
    req.set_b(6.0);
    auto result = stub.AsyncMultiply(req).get();
    TEST_ASSERT(result.first.ok(), "calc multiply call");
    TEST_ASSERT(std::abs(result.second.result() - 42.0) < 0.001, "calc multiply result");
    return true;
}

bool test_calc_divide(uint16_t port) {
    auto ch = make_channel(port);
    math::Calculator::Stub stub(ch);

    math::CalcRequest req;
    req.set_a(22.0);
    req.set_b(7.0);
    auto result = stub.AsyncDivide(req).get();
    TEST_ASSERT(result.first.ok(), "calc divide call");
    TEST_ASSERT(std::abs(result.second.result() - 22.0/7.0) < 0.001, "calc divide result");
    return true;
}

bool test_calc_divide_by_zero(uint16_t port) {
    auto ch = make_channel(port);
    math::Calculator::Stub stub(ch);

    math::CalcRequest req;
    req.set_a(1.0);
    req.set_b(0.0);
    auto result = stub.AsyncDivide(req).get();
    TEST_ASSERT(result.first.error_code() == protocomm::StatusCode::INVALID_ARGUMENT,
                "calc divide by zero error");
    return true;
}

bool test_chat_send_and_history(uint16_t port) {
    auto ch = make_channel(port);
    messaging::Chat::Stub stub(ch);

    for (int i = 0; i < 3; i++) {
        messaging::ChatMessage msg;
        msg.set_sender("user" + std::to_string(i));
        msg.set_text("Message #" + std::to_string(i));
        msg.set_timestamp(1000 + i);
        auto result = stub.AsyncSendMessage(msg).get();
        TEST_ASSERT(result.first.ok(), "chat send msg " + std::to_string(i));
        TEST_ASSERT(result.second.success(), "chat send success " + std::to_string(i));
        TEST_ASSERT(!result.second.message_id().empty(), "chat msg has id " + std::to_string(i));
    }

    messaging::HistoryRequest hist_req;
    hist_req.set_limit(10);
    auto hist_result = stub.AsyncGetHistory(hist_req).get();
    TEST_ASSERT(hist_result.first.ok(), "chat get history");
    TEST_ASSERT(hist_result.second.total() == 3, "chat history total");
    TEST_ASSERT(hist_result.second.messages_size() == 3, "chat history messages count");
    TEST_ASSERT(hist_result.second.messages(0).sender() == "user0", "chat history first sender");
    TEST_ASSERT(hist_result.second.messages(2).text() == "Message #2", "chat history last text");
    return true;
}

bool test_chat_ping(uint16_t port) {
    auto ch = make_channel(port);
    messaging::Chat::Stub stub(ch);

    messaging::PingRequest ping;
    auto now = std::chrono::duration_cast<std::chrono::milliseconds>(
        std::chrono::system_clock::now().time_since_epoch()).count();
    ping.set_sent_at(static_cast<uint64_t>(now));

    auto result = stub.AsyncPing(ping).get();
    TEST_ASSERT(result.first.ok(), "chat ping");
    TEST_ASSERT(result.second.sent_at() == ping.sent_at(), "chat ping echoes sent_at");
    TEST_ASSERT(result.second.received_at() > 0, "chat ping has received_at");
    TEST_ASSERT(result.second.received_at() >= result.second.sent_at(),
                "chat pong time >= sent time");
    return true;
}

bool test_multiple_services_same_server(uint16_t port) {
    auto ch = make_channel(port);
    Greeter::Stub greeter_stub(ch);
    math::Calculator::Stub calc_stub(ch);
    messaging::Chat::Stub chat_stub(ch);

    HelloRequest greq;
    greq.set_name("multi");
    auto gr = greeter_stub.AsyncSayHello(greq).get();
    TEST_ASSERT(gr.first.ok(), "multi-svc greeter");

    math::CalcRequest creq;
    creq.set_a(2.0);
    creq.set_b(3.0);
    auto cr = calc_stub.AsyncAdd(creq).get();
    TEST_ASSERT(cr.first.ok(), "multi-svc calculator");
    TEST_ASSERT(std::abs(cr.second.result() - 5.0) < 0.001, "multi-svc calc result");

    messaging::PingRequest preq;
    preq.set_sent_at(12345);
    auto pr = chat_stub.AsyncPing(preq).get();
    TEST_ASSERT(pr.first.ok(), "multi-svc chat ping");
    return true;
}

bool test_greeter_callback(uint16_t port) {
    auto ch = make_channel(port);
    Greeter::Stub stub(ch);

    std::promise<std::pair<protocomm::Status, HelloReply>> p;
    auto fut = p.get_future();

    HelloRequest req;
    req.set_name("cb_world");
    stub.AsyncSayHello(req,
        [&p](protocomm::Status st, HelloReply resp) {
            p.set_value({st, std::move(resp)});
        });

    auto status = fut.wait_for(std::chrono::seconds(5));
    TEST_ASSERT(status == std::future_status::ready, "callback fired in time");
    auto [st, resp] = fut.get();
    TEST_ASSERT(st.ok(), "callback got OK status");
    TEST_ASSERT(resp.message() == "Hello cb_world", "callback got correct response");
    return true;
}

bool test_handshake_mismatch(uint16_t port) {
    auto ch = protocomm::CreateChannel("127.0.0.1", port,
                                       {.handshake_header = "BAD"});
    Greeter::Stub stub(ch);

    HelloRequest req;
    auto result = stub.AsyncSayHello(req).get();
    TEST_ASSERT(!result.first.ok(), "handshake mismatch detected");
    return true;
}

bool test_interceptor_trace() {
    const uint16_t iport = 51299;
    CalculatorServiceImpl calc;

    std::mutex m;
    std::string cap_name, cap_req, cap_resp;
    bool cap_ok = false;

    protocomm::ServerBuilder builder;
    builder.AddListeningPort("127.0.0.1", iport)
           .SetHandshakeHeader("pc1")
           .RegisterService(&calc)
           .SetInterceptor([&](protocomm::ServerContext* ctx,
                               protocomm::StatusCode code,
                               const std::string&, const std::string&,
                               std::chrono::steady_clock::duration) {
               std::lock_guard lock(m);
               cap_name = ctx->method_name();
               cap_req = ctx->request_text();
               cap_resp = ctx->response_text();
               cap_ok = (code == protocomm::StatusCode::OK);
           });
    auto server = builder.BuildAndStart();
    std::this_thread::sleep_for(std::chrono::milliseconds(100));

    auto ch = make_channel(iport);
    math::Calculator::Stub stub(ch);
    math::CalcRequest req;
    req.set_a(2.0);
    req.set_b(3.0);
    auto result = stub.AsyncAdd(req).get();
    TEST_ASSERT(result.first.ok(), "interceptor: call ok");
    std::this_thread::sleep_for(std::chrono::milliseconds(50));

    std::lock_guard lock(m);
    TEST_ASSERT(cap_ok, "interceptor: status OK");
    TEST_ASSERT(cap_name == "Calculator.Add", "interceptor: method name");
    TEST_ASSERT(cap_req.find("a: 2") != std::string::npos,
                "interceptor: request decoded");
    TEST_ASSERT(cap_resp.find("result: 5") != std::string::npos,
                "interceptor: response decoded");
    server->Shutdown();
    return true;
}

int main() {
    const uint16_t port = 51234;

    GreeterServiceImpl greeter;
    CalculatorServiceImpl calculator;
    ChatServiceImpl chat;

    protocomm::ServerBuilder builder;
    builder.AddListeningPort("127.0.0.1", port)
           .SetHandshakeHeader("pc1")
           .SetIoThreadCount(2)
           .RegisterService(&greeter)
           .RegisterService(&calculator)
           .RegisterService(&chat);

    auto server = builder.BuildAndStart();
    std::this_thread::sleep_for(std::chrono::milliseconds(100));

    bool ok = true;

    std::cout << "=== Greeter Tests ===\n";
    ok = test_greeter_basic(port) && ok;
    ok = test_greeter_sync(port) && ok;
    ok = test_greeter_sequential(port) && ok;
    ok = test_greeter_error(port) && ok;
    ok = test_greeter_unimplemented(port) && ok;
    ok = test_greeter_large_payload(port) && ok;
    ok = test_greeter_callback(port) && ok;

    std::cout << "=== Calculator Tests ===\n";
    ok = test_calc_add(port) && ok;
    ok = test_calc_subtract(port) && ok;
    ok = test_calc_multiply(port) && ok;
    ok = test_calc_divide(port) && ok;
    ok = test_calc_divide_by_zero(port) && ok;

    std::cout << "=== Chat Tests ===\n";
    ok = test_chat_send_and_history(port) && ok;
    ok = test_chat_ping(port) && ok;

    std::cout << "=== Cross-service Tests ===\n";
    ok = test_multiple_services_same_server(port) && ok;
    ok = test_handshake_mismatch(port) && ok;

    std::cout << "=== Interceptor / Trace Tests ===\n";
    ok = test_interceptor_trace() && ok;

    std::cout << "\n" << pass_count << "/" << test_count << " tests passed\n";
    server->Shutdown();
    return ok ? 0 : 1;
}
