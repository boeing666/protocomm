#pragma once

#include <cstdint>
#include <memory>
#include <string>

#include "protocomm/server.h"
#include "protocomm/service.h"

namespace protocomm {

class ServerBuilder {
public:
    ServerBuilder();
    ~ServerBuilder();

    ServerBuilder& AddListeningPort(std::string address, uint16_t port);
    ServerBuilder& SetHandshakeHeader(std::string header);
    ServerBuilder& RegisterService(Service* service);
    ServerBuilder& SetIoThreadCount(int count);

    ServerBuilder& SetOnConnect(Server::OnConnectCallback cb);
    ServerBuilder& SetOnDisconnect(Server::OnDisconnectCallback cb);
    ServerBuilder& SetOnHandshake(Server::OnHandshakeCallback cb);
    ServerBuilder& SetInterceptor(Server::Interceptor cb);

    std::unique_ptr<Server> BuildAndStart();

private:
    struct Config;
    std::unique_ptr<Config> config_;
};

}  // namespace protocomm
