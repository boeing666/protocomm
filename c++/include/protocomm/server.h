#pragma once

#include <chrono>
#include <cstdint>
#include <functional>
#include <memory>
#include <string>
#include <vector>

#include "protocomm/connection.h"
#include "protocomm/server_context.h"
#include "protocomm/status.h"

namespace protocomm {

class Server {
public:
    ~Server();

    Server(const Server&) = delete;
    Server& operator=(const Server&) = delete;

    using MethodHandler = std::function<Status(ServerContext* ctx,
                                               const std::string& request,
                                               std::string* response)>;
    void RegisterMethod(uint32_t method_id, MethodHandler handler) const;

    using OnConnectCallback = std::function<void(const std::shared_ptr<Peer>&)>;
    using OnDisconnectCallback = std::function<void(const std::shared_ptr<Peer>&)>;

    using OnHandshakeCallback = std::function<bool(const std::shared_ptr<Peer>&,
                                                    const std::string& header)>;

    using Interceptor =
        std::function<void(ServerContext* ctx, StatusCode code,
                           const std::string& request, const std::string& response,
                           std::chrono::steady_clock::duration latency)>;

    void SetOnConnect(OnConnectCallback cb) const;
    void SetOnDisconnect(OnDisconnectCallback cb) const;
    void SetOnHandshake(OnHandshakeCallback cb) const;
    void SetInterceptor(Interceptor cb) const;

    std::vector<std::shared_ptr<Peer>> GetConnections() const;
    void Broadcast(uint32_t method_id, const std::string& payload) const;

    void Wait() const;

    void Shutdown() const;

private:
    friend class ServerBuilder;
    Server();

    struct Impl;
    std::unique_ptr<Impl> impl_;
};

}  // namespace protocomm
