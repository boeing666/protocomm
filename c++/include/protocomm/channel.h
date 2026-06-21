#pragma once

#include <cstdint>
#include <functional>
#include <future>
#include <memory>
#include <string>
#include <utility>

#include "protocomm/status.h"

namespace protocomm {

struct ChannelConfig {
    std::string handshake_header = "pc1";

    std::function<void(uint32_t method_id, const std::string& payload)> on_push;
};

using ResponseCallback = std::function<void(Status, std::string)>;
using ConnectCallback = std::function<void(Status)>;

class Channel {
public:
    Channel(std::string host, uint16_t port, ChannelConfig config);
    ~Channel();

    Channel(const Channel&) = delete;
    Channel& operator=(const Channel&) = delete;

    Status UnaryCall(uint32_t method_id,
                     const std::string& request,
                     std::string* response);

    std::future<std::pair<Status, std::string>>
    AsyncUnaryCall(uint32_t method_id, std::string request);

    void AsyncUnaryCall(uint32_t method_id, std::string request,
                        ResponseCallback callback);

    Status Connect();
    void ConnectAsync(ConnectCallback cb);

private:
    struct Impl;
    std::unique_ptr<Impl> impl_;
};

std::shared_ptr<Channel> CreateChannel(std::string host, uint16_t port,
                                       ChannelConfig config = {});

}
