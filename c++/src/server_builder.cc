#include "protocomm/server_builder.h"
#include "server_impl.h"

#include <vector>

namespace protocomm {

struct ServerBuilder::Config {
    std::string address = "0.0.0.0";
    uint16_t port = 0;
    std::string handshake_header = "pc1";
    std::vector<Service*> services;
    int io_thread_count = 1;
    Server::OnConnectCallback on_connect;
    Server::OnDisconnectCallback on_disconnect;
    Server::OnHandshakeCallback on_handshake;
    Server::Interceptor interceptor;
};

ServerBuilder::ServerBuilder() : config_(std::make_unique<Config>()) {}
ServerBuilder::~ServerBuilder() = default;

ServerBuilder& ServerBuilder::AddListeningPort(std::string address,
                                                uint16_t port) {
    config_->address = std::move(address);
    config_->port = port;
    return *this;
}

ServerBuilder& ServerBuilder::SetHandshakeHeader(std::string header) {
    config_->handshake_header = std::move(header);
    return *this;
}

ServerBuilder& ServerBuilder::RegisterService(Service* service) {
    config_->services.push_back(service);
    return *this;
}

ServerBuilder& ServerBuilder::SetIoThreadCount(int count) {
    config_->io_thread_count = count;
    return *this;
}

ServerBuilder& ServerBuilder::SetOnConnect(Server::OnConnectCallback cb) {
    config_->on_connect = std::move(cb);
    return *this;
}

ServerBuilder& ServerBuilder::SetOnDisconnect(Server::OnDisconnectCallback cb) {
    config_->on_disconnect = std::move(cb);
    return *this;
}

ServerBuilder& ServerBuilder::SetOnHandshake(Server::OnHandshakeCallback cb) {
    config_->on_handshake = std::move(cb);
    return *this;
}

ServerBuilder& ServerBuilder::SetInterceptor(Server::Interceptor cb) {
    config_->interceptor = std::move(cb);
    return *this;
}

std::unique_ptr<Server> ServerBuilder::BuildAndStart() {
    auto server = std::unique_ptr<Server>(new Server());

    server->impl_->bind_address = config_->address;
    server->impl_->bind_port = config_->port;
    server->impl_->handshake_header = config_->handshake_header;
    server->impl_->io_thread_count = config_->io_thread_count;

    if (config_->on_connect) server->SetOnConnect(std::move(config_->on_connect));
    if (config_->on_disconnect) server->SetOnDisconnect(std::move(config_->on_disconnect));
    if (config_->on_handshake) server->SetOnHandshake(std::move(config_->on_handshake));
    if (config_->interceptor) server->SetInterceptor(std::move(config_->interceptor));

    for (auto* svc : config_->services) {
        svc->RegisterWith(server.get());
    }

    server->impl_->Start();
    return server;
}

}  // namespace protocomm
