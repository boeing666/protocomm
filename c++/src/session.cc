#include "session.h"
#include "transport.h"

#include <boost/asio/co_spawn.hpp>
#include <boost/asio/detached.hpp>
#include <boost/asio/post.hpp>
#include <boost/asio/use_awaitable.hpp>

namespace protocomm {

Session::Session(boost::asio::ip::tcp::socket socket,
                 const HandlerMap& handlers,
                 std::string handshake_header,
                 uint64_t connection_id,
                 OnConnectCb on_connect,
                 OnDisconnectCb on_disconnect,
                 OnHandshakeCb on_handshake)
    : socket_(std::move(socket)),
      strand_(boost::asio::make_strand(socket_.get_executor())),
      handlers_(handlers),
      handshake_header_(std::move(handshake_header)),
      connection_id_(connection_id),
      on_connect_(std::move(on_connect)),
      on_disconnect_(std::move(on_disconnect)),
      on_handshake_(std::move(on_handshake)) {
    boost::system::error_code ec;
    auto ep = socket_.remote_endpoint(ec);
    if (!ec) {
        peer_address_ = ep.address().to_string() + ":" + std::to_string(ep.port());
    }
}

void Session::EnqueueWrite(FrameHeader hdr, std::string payload) {
    auto self = shared_from_this();
    boost::asio::post(strand_,
        [self, hdr, payload = std::move(payload)]() mutable {
            self->write_queue_.emplace_back(std::move(hdr), std::move(payload));
            if (!self->write_pump_active_) {
                self->write_pump_active_ = true;
                boost::asio::co_spawn(self->strand_,
                    self->WritePump(), boost::asio::detached);
            }
        });
}

boost::asio::awaitable<void> Session::WritePump() {
    auto self = shared_from_this();
    while (!write_queue_.empty()) {
        auto [hdr, payload] = std::move(write_queue_.front());
        write_queue_.pop_front();

        Status ws = co_await AsyncTransport::WriteFrame(socket_, hdr, payload);
        if (!ws.ok()) {
            write_queue_.clear();
            break;
        }
    }
    write_pump_active_ = false;
}

void Session::Send(uint32_t method_id, const std::string& payload) {
    FrameHeader hdr;
    hdr.type = FrameType::kPush;
    hdr.method_id = method_id;
    hdr.payload_size = static_cast<uint32_t>(payload.size());
    EnqueueWrite(hdr, payload);
}

void Session::Close() {
    boost::asio::post(strand_, [self = shared_from_this()]() {
        boost::system::error_code ec;
        self->socket_.close(ec);
    });
}

boost::asio::awaitable<void> Session::Run() {
    auto self = std::dynamic_pointer_cast<Peer>(shared_from_this());

    if (on_connect_) on_connect_(self);

    struct ScopeGuard {
        OnDisconnectCb& cb;
        std::shared_ptr<Peer> peer;
        ~ScopeGuard() { if (cb) cb(peer); }
    } guard{on_disconnect_, self};

    if (!handshake_header_.empty()) {
        std::string received_header;
        Status hs = co_await AsyncTransport::ReadHandshakeRaw(
            socket_, handshake_header_.size(), received_header);
        if (!hs.ok()) co_return;

        if (received_header != handshake_header_) co_return;

        if (on_handshake_ && !on_handshake_(self, received_header)) {
            co_return;
        }

        hs = co_await AsyncTransport::WriteHandshake(socket_, handshake_header_);
        if (!hs.ok()) co_return;
    }

    for (;;) {
        FrameHeader req_hdr;
        std::string req_payload;
        Status rs = co_await AsyncTransport::ReadFrame(socket_, req_hdr, req_payload);
        if (!rs.ok()) co_return;

        if (req_hdr.type != FrameType::kRequest) co_return;

        ServerContext ctx;
        ctx.peer_address_ = peer_address_;
        ctx.method_id_ = req_hdr.method_id;
        ctx.call_id_ = req_hdr.call_id;

        std::string resp_payload;
        StatusCode resp_code = StatusCode::OK;

        auto it = handlers_.find(req_hdr.method_id);
        if (it == handlers_.end()) {
            resp_code = StatusCode::UNIMPLEMENTED;
        } else {
            Status handler_status = it->second(&ctx, req_payload, &resp_payload);
            if (!handler_status.ok()) {
                resp_code = handler_status.error_code();
                resp_payload.clear();
            }
        }

        FrameHeader resp_hdr;
        resp_hdr.type = FrameType::kResponse;
        resp_hdr.status_code = static_cast<uint16_t>(static_cast<int>(resp_code));
        resp_hdr.call_id = req_hdr.call_id;
        resp_hdr.method_id = req_hdr.method_id;
        resp_hdr.payload_size = static_cast<uint32_t>(resp_payload.size());

        EnqueueWrite(resp_hdr, std::move(resp_payload));
    }
}

}  // namespace protocomm
