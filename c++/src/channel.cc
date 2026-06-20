#include "protocomm/channel.h"
#include "frame.h"
#include "transport.h"

#include <atomic>
#include <deque>
#include <future>
#include <mutex>
#include <thread>
#include <unordered_map>
#include <utility>

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

struct Channel::Impl {
    std::string host;
    uint16_t port = 0;
    ChannelConfig config;

    boost::asio::io_context io_context;
    boost::asio::executor_work_guard<boost::asio::io_context::executor_type> work_guard;
    boost::asio::ip::tcp::socket socket;
    boost::asio::strand<boost::asio::any_io_executor> strand;
    std::thread io_thread;

    std::mutex connect_mu;
    std::atomic<bool> connected{false};
    std::atomic<bool> stopped{false};
    uint32_t next_call_id = 1;
    std::unordered_map<uint32_t, ResponseCallback> pending;

    std::deque<std::pair<FrameHeader, std::string>> write_queue;
    bool write_pump_active = false;

    Impl()
        : work_guard(boost::asio::make_work_guard(io_context)),
          socket(io_context),
          strand(boost::asio::make_strand(io_context.get_executor())) {}

    Status EnsureConnected();
    void StartIoThread();
    void StartReader();
    boost::asio::awaitable<void> WritePump();
    boost::asio::awaitable<void> ReadLoop();
    void FailAllPending(Status st);
    void Shutdown();
};

Status Channel::Impl::EnsureConnected() {
    if (connected.load(std::memory_order_acquire)) return {};

    std::lock_guard lock(connect_mu);
    if (connected.load(std::memory_order_relaxed)) return {};
    if (stopped.load(std::memory_order_acquire))
        return {StatusCode::UNAVAILABLE, "channel stopped"};

    boost::asio::ip::tcp::resolver resolver(io_context);
    boost::system::error_code ec;
    auto endpoints = resolver.resolve(host, std::to_string(port), ec);
    if (ec) return {StatusCode::UNAVAILABLE, "resolve: " + ec.message()};

    boost::asio::connect(socket, endpoints, ec);
    if (ec) return {StatusCode::UNAVAILABLE, "connect: " + ec.message()};

    socket.set_option(boost::asio::ip::tcp::no_delay(true), ec);

    if (!config.handshake_header.empty()) {
        Status hs = SyncTransport::WriteHandshake(socket, config.handshake_header);
        if (!hs.ok()) {
            boost::system::error_code ec_close;
            socket.close(ec_close);
            return hs;
        }
        hs = SyncTransport::ReadAndVerifyHandshake(socket, config.handshake_header);
        if (!hs.ok()) {
            boost::system::error_code ec_close;
            socket.close(ec_close);
            return hs;
        }
    }

    StartIoThread();
    StartReader();
    connected.store(true, std::memory_order_release);
    return {};
}

void Channel::Impl::StartIoThread() {
    io_thread = std::thread([this]() {
        try {
            io_context.run();
        } catch (...) {
        }
    });
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
            FailAllPending(rs);
            co_return;
        }

        if (hdr.type == FrameType::kPush) {
            if (config.on_push) {
                config.on_push(hdr.method_id, payload);
            }
            continue;
        }

        if (hdr.type != FrameType::kResponse) {
            FailAllPending({StatusCode::INTERNAL, "unexpected frame type"});
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
            cb(Status{code, "remote error"}, {});
        } else {
            cb(Status{}, std::move(payload));
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
            FailAllPending(ws);
            break;
        }
    }
    write_pump_active = false;
}

void Channel::Impl::FailAllPending(Status st) {
    auto snapshot = std::move(pending);
    pending.clear();
    for (auto& [call_id, cb] : snapshot) {
        if (cb) cb(st, {});
    }
}

void Channel::Impl::Shutdown() {
    if (stopped.exchange(true)) return;

    if (io_thread.joinable()) {
        boost::asio::post(strand, [this]() {
            boost::system::error_code ec;
            socket.close(ec);
            FailAllPending({StatusCode::CANCELLED, "channel shutdown"});
        });
        work_guard.reset();
        io_thread.join();
    } else {
        FailAllPending({StatusCode::CANCELLED, "channel shutdown"});
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

void Channel::AsyncUnaryCall(uint32_t method_id, std::string request,
                              ResponseCallback callback) {
    Status cs = impl_->EnsureConnected();
    if (!cs.ok()) {
        if (callback) callback(cs, {});
        return;
    }

    boost::asio::post(impl_->strand,
        [impl = impl_.get(), method_id, request = std::move(request),
         callback = std::move(callback)]() mutable {
            if (impl->stopped.load(std::memory_order_acquire)) {
                if (callback)
                    callback({StatusCode::CANCELLED, "channel shutdown"}, {});
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

std::shared_ptr<Channel> CreateChannel(std::string host, uint16_t port,
                                       ChannelConfig config) {
    return std::make_shared<Channel>(std::move(host), port, std::move(config));
}

}  // namespace protocomm
