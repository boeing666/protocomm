#pragma once

#include <cstdint>
#include <string>
#include <utility>

namespace protocomm {

class ServerContext {
public:
    [[nodiscard]] const std::string& peer_address() const noexcept {
        return peer_address_;
    }

    [[nodiscard]] uint32_t method_id() const noexcept { return method_id_; }
    [[nodiscard]] uint32_t call_id() const noexcept { return call_id_; }

    [[nodiscard]] bool tracing() const noexcept { return trace_; }
    [[nodiscard]] const char* method_name() const noexcept { return method_name_; }
    [[nodiscard]] const std::string& request_text() const noexcept {
        return request_text_;
    }
    [[nodiscard]] const std::string& response_text() const noexcept {
        return response_text_;
    }

    void set_method_name(const char* name) noexcept { method_name_ = name; }
    void set_request_text(std::string text) { request_text_ = std::move(text); }
    void set_response_text(std::string text) { response_text_ = std::move(text); }

private:
    friend class Session;
    std::string peer_address_;
    uint32_t method_id_ = 0;
    uint32_t call_id_ = 0;

    bool trace_ = false;
    const char* method_name_ = "";
    std::string request_text_;
    std::string response_text_;
};

}
