#pragma once

#include <cstdint>
#include <memory>
#include <string>

namespace protocomm {

class Peer {
public:
    virtual ~Peer() = default;

    virtual uint64_t id() const = 0;
    virtual const std::string& peer_address() const = 0;

    virtual void Send(uint32_t method_id, const std::string& payload) = 0;

    virtual void Close() = 0;
};

}  // namespace protocomm
