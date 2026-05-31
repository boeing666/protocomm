#pragma once

#include <cstdint>
#include <string>

namespace protocomm {

class ServerContext {
public:
    [[nodiscard]] const std::string& peer_address() const noexcept {
        return peer_address_;
    }

    [[nodiscard]] uint32_t method_id() const noexcept { return method_id_; }
    [[nodiscard]] uint32_t call_id() const noexcept { return call_id_; }

private:
    friend class Session;
    std::string peer_address_;
    uint32_t method_id_ = 0;
    uint32_t call_id_ = 0;
};

}  // namespace protocomm
