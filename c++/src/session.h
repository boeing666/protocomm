#pragma once

#include <chrono>
#include <cstdint>
#include <deque>
#include <functional>
#include <memory>
#include <string>
#include <unordered_map>
#include <utility>

#include <boost/asio/awaitable.hpp>
#include <boost/asio/io_context.hpp>
#include <boost/asio/ip/tcp.hpp>
#include <boost/asio/strand.hpp>

#include "protocomm/connection.h"
#include "protocomm/server_context.h"
#include "protocomm/status.h"
#include "frame.h"

namespace protocomm {

class Session : public Peer, public std::enable_shared_from_this<Session> {
public:
    using MethodHandler = std::function<Status(ServerContext* ctx,
                                               const std::string& request,
                                               std::string* response)>;
    using HandlerMap = std::unordered_map<uint32_t, MethodHandler>;

    using OnConnectCb = std::function<void(const std::shared_ptr<Peer>&)>;
    using OnDisconnectCb = std::function<void(const std::shared_ptr<Peer>&)>;
    using OnHandshakeCb = std::function<bool(const std::shared_ptr<Peer>&, const std::string&)>;
    using InterceptorCb =
        std::function<void(ServerContext*, StatusCode, const std::string&,
                           const std::string&, std::chrono::steady_clock::duration)>;

    Session(boost::asio::ip::tcp::socket socket,
            const HandlerMap& handlers,
            std::string handshake_header,
            uint64_t connection_id,
            OnConnectCb on_connect,
            OnDisconnectCb on_disconnect,
            OnHandshakeCb on_handshake,
            InterceptorCb interceptor);

    boost::asio::awaitable<void> Run();

    uint64_t id() const override { return connection_id_; }
    const std::string& peer_address() const override { return peer_address_; }
    void Send(uint32_t method_id, const std::string& payload) override;
    void Close() override;

private:
    void EnqueueWrite(FrameHeader hdr, std::string payload);
    boost::asio::awaitable<void> WritePump();

    boost::asio::ip::tcp::socket socket_;
    boost::asio::strand<boost::asio::any_io_executor> strand_;
    const HandlerMap& handlers_;
    std::string handshake_header_;
    std::string peer_address_;
    uint64_t connection_id_;

    OnConnectCb on_connect_;
    OnDisconnectCb on_disconnect_;
    OnHandshakeCb on_handshake_;
    InterceptorCb interceptor_;

    std::deque<std::pair<FrameHeader, std::string>> write_queue_;
    bool write_pump_active_ = false;
};

}  // namespace protocomm
