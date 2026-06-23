#pragma once

#include <cstddef>
#include <cstdint>
#include <functional>
#include <future>
#include <memory>
#include <string>
#include <utility>

#include "protocomm/status.h"

namespace protocomm {

// How response/push callbacks are delivered:
//   Inline — invoked directly on the protocomm io thread (default; standalone apps).
//   Manual — queued and invoked only when the owner calls Channel::RunCallbacks(), on
//            that thread (Steamworks SteamAPI_RunCallbacks style). Lets an embedder run
//            every callback on its own thread (e.g. a single-threaded game loop) without
//            its own marshaling layer.
enum class Dispatch { Inline, Manual };

struct ChannelConfig {
    std::string handshake_header = "pc1";

    Dispatch dispatch = Dispatch::Inline;

    std::function<void(uint32_t method_id, const std::string& payload)> on_push;
};

using ResponseCallback = std::function<void(Status, std::string)>;
using ConnectCallback = std::function<void(Status)>;

class ChannelInterface {
public:
    virtual ~ChannelInterface() = default;

    virtual Status UnaryCall(uint32_t method_id,
                             const std::string& request,
                             std::string* response) = 0;

    virtual std::future<std::pair<Status, std::string>>
    AsyncUnaryCall(uint32_t method_id, std::string request) = 0;

    virtual void AsyncUnaryCall(uint32_t method_id, std::string request,
                                ResponseCallback callback) = 0;
};

class Channel : public ChannelInterface {
public:
    Channel(std::string host, uint16_t port, ChannelConfig config);
    ~Channel() override;

    Channel(const Channel&) = delete;
    Channel& operator=(const Channel&) = delete;

    Status UnaryCall(uint32_t method_id,
                     const std::string& request,
                     std::string* response) override;

    std::future<std::pair<Status, std::string>>
    AsyncUnaryCall(uint32_t method_id, std::string request) override;

    void AsyncUnaryCall(uint32_t method_id, std::string request,
                        ResponseCallback callback) override;

    Status Connect();
    void ConnectAsync(ConnectCallback cb);

    std::size_t RunCallbacks(std::size_t max = static_cast<std::size_t>(-1));
private:
    struct Impl;
    std::unique_ptr<Impl> impl_;
};

std::shared_ptr<Channel> CreateChannel(std::string host, uint16_t port,
                                       ChannelConfig config = {});

}
