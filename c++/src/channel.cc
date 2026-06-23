#include "protocomm/channel.h"
#include "frame.h"
#include "mpsc_queue.h"
#include "transport.h"

#include <atomic>
#include <deque>
#include <future>
#include <mutex>
#include <thread>
#include <unordered_map>
#include <utility>
#include <vector>

#include <boost/asio/co_spawn.hpp>
#include <boost/asio/connect.hpp>
#include <boost/asio/detached.hpp>
#include <boost/asio/executor_work_guard.hpp>
#include <boost/asio/io_context.hpp>
#include <boost/asio/ip/tcp.hpp>
#include <boost/asio/post.hpp>
#include <boost/asio/strand.hpp>
#include <boost/asio/use_awaitable.hpp>

namespace protocomm {

struct Completion {
    std::atomic<Completion*> mpsc_next{nullptr};
    enum class Kind { Response, Deferred } kind = Kind::Deferred;
    ResponseCallback resp_cb;
    Status status;
    std::string payload;
    std::function<void()> thunk;
};

struct Channel::Impl {
    std::string host;
    uint16_t port = 0;
    ChannelConfig config;
    ClientInterceptor interceptor;

    boost::asio::io_context io_context;
    boost::asio::executor_work_guard<boost::asio::io_context::executor_type> work_guard;
    boost::asio::ip::tcp::socket socket;
    boost::asio::strand<boost::asio::any_io_executor> strand;
    std::thread io_thread;

    std::mutex connect_mu;
    bool io_thread_started = false;
    std::atomic<bool> connected{false};
    std::atomic<bool> stopped{false};
    uint32_t next_call_id = 1;
    std::unordered_map<uint32_t, ResponseCallback> pending;

    std::deque<std::pair<FrameHeader, std::string>> write_queue;
    bool write_pump_active = false;

    enum class ConnState { Idle, Connecting, Connected, Closed };
    ConnState conn_state = ConnState::Idle;
    std::vector<ConnectCallback> connect_waiters;

    detail::MpscQueue<Completion> cb_queue;

    Impl()
        : work_guard(boost::asio::make_work_guard(io_context)),
          socket(io_context),
          strand(boost::asio::make_strand(io_context.get_executor())) {}

    Status EnsureConnected();
    void EnsureIoThread();
    void RequestConnect(ConnectCallback cb);
    boost::asio::awaitable<void> ConnectCoro();
    void finish(Status st);
    void StartReader();
    boost::asio::awaitable<void> WritePump();
    boost::asio::awaitable<void> ReadLoop();
    void FailAllPending(Status st, bool defer);
    void post_response(ResponseCallback cb, Status st, std::string payload);
    void post_deferred(std::function<void()> fn);
    void Shutdown();
};

void Channel::Impl::post_response(ResponseCallback cb, Status st, std::string payload) {
    if (!cb) return;
    if (config.dispatch == Dispatch::Inline) {
        cb(std::move(st), std::move(payload));
        return;
    }
    auto* c = new Completion();
    c->kind = Completion::Kind::Response;
    c->resp_cb = std::move(cb);
    c->status = std::move(st);
    c->payload = std::move(payload);
    cb_queue.push(c);
}

void Channel::Impl::post_deferred(std::function<void()> fn) {
    if (config.dispatch == Dispatch::Inline) {
        fn();
        return;
    }
    auto* c = new Completion();
    c->kind = Completion::Kind::Deferred;
    c->thunk = std::move(fn);
    cb_queue.push(c);
}

Status Channel::Impl::EnsureConnected() {
    if (connected.load(std::memory_order_acquire)) return {};

    if (strand.running_in_this_thread()) {
        return {StatusCode::FAILED_PRECONDITION,
                "synchronous connect attempted from io thread; "
                "channel not yet connected"};
    }

    if (stopped.load(std::memory_order_acquire))
        return {StatusCode::UNAVAILABLE, "channel stopped"};

    auto p = std::make_shared<std::promise<Status>>();
    auto f = p->get_future();
    RequestConnect([p](Status s) { p->set_value(s); });

    try {
        return f.get();
    } catch (const std::future_error&) {
        return {StatusCode::UNAVAILABLE, "channel stopped"};
    }
}

void Channel::Impl::EnsureIoThread() {
    std::lock_guard<std::mutex> lock(connect_mu);
    if (io_thread_started) return;
    if (stopped.load(std::memory_order_acquire)) return;
    io_thread = std::thread([this]() {
        try {
            io_context.run();
        } catch (...) {
        }
    });
    io_thread_started = true;
}

void Channel::Impl::RequestConnect(ConnectCallback cb) {
    if (stopped.load(std::memory_order_acquire)) {
        if (cb) cb({StatusCode::UNAVAILABLE, "channel stopped"});
        return;
    }

    EnsureIoThread();

    {
        std::lock_guard<std::mutex> lock(connect_mu);
        if (!io_thread_started) {
            if (cb) cb({StatusCode::UNAVAILABLE, "channel stopped"});
            return;
        }
    }

    boost::asio::post(strand, [this, cb = std::move(cb)]() mutable {
        if (conn_state == ConnState::Closed ||
            stopped.load(std::memory_order_acquire)) {
            if (cb) cb({StatusCode::UNAVAILABLE, "channel stopped"});
            return;
        }
        if (conn_state == ConnState::Connected) {
            if (cb) cb({});
            return;
        }
        if (cb) connect_waiters.push_back(std::move(cb));
        if (conn_state == ConnState::Idle) {
            conn_state = ConnState::Connecting;
            boost::asio::co_spawn(strand, ConnectCoro(), boost::asio::detached);
        }
    });
}

boost::asio::awaitable<void> Channel::Impl::ConnectCoro() {
    using boost::asio::use_awaitable;
    namespace ip = boost::asio::ip;

    ip::tcp::resolver resolver(io_context);

    try {
        auto endpoints = co_await resolver.async_resolve(
            host, std::to_string(port), use_awaitable);

        co_await boost::asio::async_connect(socket, endpoints, use_awaitable);

        boost::system::error_code ec;
        socket.set_option(ip::tcp::no_delay(true), ec);

        if (!config.handshake_header.empty()) {
            Status hs = co_await AsyncTransport::WriteHandshake(
                socket, config.handshake_header);
            if (!hs.ok()) { finish(hs); co_return; }

            hs = co_await AsyncTransport::ReadHandshake(
                socket, config.handshake_header);
            if (!hs.ok()) { finish(hs); co_return; }
        }
    } catch (const boost::system::system_error& e) {
        finish({StatusCode::UNAVAILABLE,
                std::string("connect: ") + e.code().message()});
        co_return;
    }

    if (conn_state == ConnState::Closed ||
        stopped.load(std::memory_order_acquire)) {
        finish({StatusCode::CANCELLED, "channel shutdown"});
        co_return;
    }

    conn_state = ConnState::Connected;
    StartReader();
    connected.store(true, std::memory_order_release);
    finish({});
}

void Channel::Impl::finish(Status st) {
    if (!st.ok() && conn_state != ConnState::Closed) {
        conn_state = ConnState::Idle;
    }
    auto waiters = std::move(connect_waiters);
    connect_waiters.clear();
    for (auto& cb : waiters) {
        if (cb) post_deferred([cb = std::move(cb), st]() mutable { cb(st); });
    }
}

void Channel::Impl::StartReader() {
    boost::asio::co_spawn(strand, ReadLoop(), boost::asio::detached);
}

boost::asio::awaitable<void> Channel::Impl::ReadLoop() {
    for (;;) {
        FrameHeader hdr;
        std::string payload;
        Status rs = co_await AsyncTransport::ReadFrame(socket, hdr, payload);
        if (!rs.ok()) {
            FailAllPending(rs, /*defer*/ true);
            co_return;
        }

        if (hdr.type == FrameType::kPush) {
            if (config.on_push) {
                post_deferred([this, mid = hdr.method_id, payload = std::move(payload)]() {
                    config.on_push(mid, payload);
                });
            }
            continue;
        }

        if (hdr.type != FrameType::kResponse) {
            FailAllPending({StatusCode::INTERNAL, "unexpected frame type"}, /*defer*/ true);
            co_return;
        }

        ResponseCallback cb;
        auto it = pending.find(hdr.call_id);
        if (it != pending.end()) {
            cb = std::move(it->second);
            pending.erase(it);
        }
        if (!cb) {
            continue;
        }

        auto code = static_cast<StatusCode>(hdr.status_code);
        if (code != StatusCode::OK) {
            post_response(std::move(cb), Status{code, "remote error"}, {});
        } else {
            post_response(std::move(cb), Status{}, std::move(payload));
        }
    }
}

boost::asio::awaitable<void> Channel::Impl::WritePump() {
    while (!write_queue.empty()) {
        auto [hdr, payload] = std::move(write_queue.front());
        write_queue.pop_front();

        Status ws = co_await AsyncTransport::WriteFrame(socket, hdr, payload);
        if (!ws.ok()) {
            write_queue.clear();
            FailAllPending(ws, /*defer*/ true);
            break;
        }
    }
    write_pump_active = false;
}

void Channel::Impl::FailAllPending(Status st, bool defer) {
    auto snapshot = std::move(pending);
    pending.clear();
    for (auto& [call_id, cb] : snapshot) {
        if (!cb) continue;
        if (defer) {
            post_response(std::move(cb), st, {});
        } else {
            cb(st, {});
        }
    }
}

void Channel::Impl::Shutdown() {
    if (stopped.exchange(true)) return;

    bool started;
    {
        std::lock_guard<std::mutex> lock(connect_mu);
        started = io_thread_started;
    }

    if (started) {
        boost::asio::post(strand, [this]() {
            conn_state = ConnState::Closed;
            boost::system::error_code ec;
            socket.close(ec);
            FailAllPending({StatusCode::CANCELLED, "channel shutdown"}, /*defer*/ false);
            auto waiters = std::move(connect_waiters);
            connect_waiters.clear();
            for (auto& cb : waiters) {
                if (cb) cb({StatusCode::CANCELLED, "channel shutdown"});
            }
        });
        work_guard.reset();
        io_thread.join();
    } else {
        conn_state = ConnState::Closed;
        FailAllPending({StatusCode::CANCELLED, "channel shutdown"}, /*defer*/ false);
        connect_waiters.clear();
    }

    // Free any Manual-dispatch completions queued but never drained — the channel is going
    // away and there is no consumer. The io thread is joined / never ran, so no producers race.
    while (Completion* c = cb_queue.pop()) {
        delete c;
    }
}

Channel::Channel(std::string host, uint16_t port, ChannelConfig config)
    : impl_(std::make_unique<Impl>()) {
    impl_->host = std::move(host);
    impl_->port = port;
    impl_->config = std::move(config);
}

Channel::~Channel() {
    impl_->Shutdown();
}

Status Channel::Connect() {
    return impl_->EnsureConnected();
}

void Channel::ConnectAsync(ConnectCallback cb) {
    impl_->RequestConnect(std::move(cb));
}

void Channel::SetInterceptor(ClientInterceptor cb) {
    impl_->interceptor = std::move(cb);
}

bool Channel::Tracing() const {
    return static_cast<bool>(impl_->interceptor);
}

void Channel::OnTrace(const char* method, const std::string& request_text,
                      const std::string& response_text, const Status& status) {
    if (impl_->interceptor) {
        impl_->interceptor(method, request_text, response_text, status);
    }
}

void Channel::AsyncUnaryCall(uint32_t method_id, std::string request,
                              ResponseCallback callback) {
    Status cs = impl_->EnsureConnected();
    if (!cs.ok()) {
        impl_->post_response(std::move(callback), std::move(cs), {});
        return;
    }

    boost::asio::post(impl_->strand,
        [impl = impl_.get(), method_id, request = std::move(request),
         callback = std::move(callback)]() mutable {
            if (impl->stopped.load(std::memory_order_acquire)) {
                impl->post_response(std::move(callback),
                                    {StatusCode::CANCELLED, "channel shutdown"}, {});
                return;
            }

            uint32_t call_id = impl->next_call_id++;
            impl->pending.emplace(call_id, std::move(callback));

            FrameHeader hdr;
            hdr.type = FrameType::kRequest;
            hdr.call_id = call_id;
            hdr.method_id = method_id;
            hdr.payload_size = static_cast<uint32_t>(request.size());

            impl->write_queue.emplace_back(hdr, std::move(request));
            if (!impl->write_pump_active) {
                impl->write_pump_active = true;
                boost::asio::co_spawn(impl->strand, impl->WritePump(),
                                      boost::asio::detached);
            }
        });
}

std::future<std::pair<Status, std::string>>
Channel::AsyncUnaryCall(uint32_t method_id, std::string request) {
    auto promise = std::make_shared<std::promise<std::pair<Status, std::string>>>();
    auto future = promise->get_future();
    AsyncUnaryCall(method_id, std::move(request),
        [promise](Status st, std::string bytes) {
            promise->set_value({st, std::move(bytes)});
        });
    return future;
}

Status Channel::UnaryCall(uint32_t method_id,
                          const std::string& request,
                          std::string* response) {
    auto future = AsyncUnaryCall(method_id, request);
    auto [st, bytes] = future.get();
    if (!st.ok()) return st;
    *response = std::move(bytes);
    return {};
}

std::size_t Channel::RunCallbacks(std::size_t max) {
    std::size_t ran = 0;
    while (ran < max) {
        Completion* c = impl_->cb_queue.pop();
        if (c == nullptr) {
            break;
        }
        if (c->kind == Completion::Kind::Response) {
            if (c->resp_cb) c->resp_cb(std::move(c->status), std::move(c->payload));
        } else if (c->thunk) {
            c->thunk();
        }
        delete c;
        ++ran;
    }
    return ran;
}

std::shared_ptr<Channel> CreateChannel(std::string host, uint16_t port,
                                       ChannelConfig config) {
    return std::make_shared<Channel>(std::move(host), port, std::move(config));
}

}
