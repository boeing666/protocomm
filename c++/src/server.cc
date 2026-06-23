#include "server_impl.h"

namespace protocomm {

Server::Server() : impl_(std::make_unique<Impl>()) {}
Server::~Server() { Shutdown(); }

void Server::RegisterMethod(uint32_t method_id, MethodHandler handler) const {
    std::lock_guard lock(impl_->handler_mu);
    impl_->handlers[method_id] = std::move(handler);
}

void Server::SetOnConnect(OnConnectCallback cb) const {
    impl_->on_connect = std::move(cb);
}

void Server::SetOnDisconnect(OnDisconnectCallback cb) const {
    impl_->on_disconnect = std::move(cb);
}

void Server::SetOnHandshake(OnHandshakeCallback cb) const {
    impl_->on_handshake = std::move(cb);
}

void Server::SetInterceptor(Interceptor cb) const {
    impl_->interceptor = std::move(cb);
}

std::vector<std::shared_ptr<Peer>> Server::GetConnections() const {
    std::lock_guard lock(impl_->sessions_mu);
    std::vector<std::shared_ptr<Peer>> result;
    result.reserve(impl_->sessions.size());
    for (auto& [id, session] : impl_->sessions) {
        result.push_back(session);
    }
    return result;
}

void Server::Broadcast(uint32_t method_id, const std::string& payload) const {
    std::vector<std::shared_ptr<Session>> snapshot;
    {
        std::lock_guard lock(impl_->sessions_mu);
        snapshot.reserve(impl_->sessions.size());
        for (auto& [id, session] : impl_->sessions) {
            snapshot.push_back(session);
        }
    }
    for (auto& session : snapshot) {
        session->Send(method_id, payload);
    }
}

void Server::Wait() const {
    for (auto& t : impl_->threads) {
        if (t.joinable()) t.join();
    }
}

void Server::Shutdown() const {
    if (!impl_->running) return;
    impl_->running = false;
    impl_->work_guard.reset();
    impl_->io_context.stop();
    for (auto& t : impl_->threads) {
        if (t.joinable()) t.join();
    }
    impl_->threads.clear();
}

}  // namespace protocomm
