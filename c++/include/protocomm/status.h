#pragma once

#include <string>
#include <string_view>

namespace protocomm {

enum class StatusCode : int {
    OK = 0,
    CANCELLED = 1,
    UNKNOWN = 2,
    INVALID_ARGUMENT = 3,
    DEADLINE_EXCEEDED = 4,
    NOT_FOUND = 5,
    ALREADY_EXISTS = 6,
    PERMISSION_DENIED = 7,
    RESOURCE_EXHAUSTED = 8,
    FAILED_PRECONDITION = 9,
    ABORTED = 10,
    OUT_OF_RANGE = 11,
    UNIMPLEMENTED = 12,
    INTERNAL = 13,
    UNAVAILABLE = 14,
    DATA_LOSS = 15,
    UNAUTHENTICATED = 16,
};

class Status {
public:
    Status() noexcept = default;

    Status(StatusCode code, std::string message) noexcept
        : code_(code), message_(std::move(message)) {}

    [[nodiscard]] StatusCode error_code() const noexcept { return code_; }
    [[nodiscard]] const std::string& error_message() const noexcept { return message_; }
    [[nodiscard]] bool ok() const noexcept { return code_ == StatusCode::OK; }

    static Status OK_status() noexcept { return {}; }

private:
    StatusCode code_ = StatusCode::OK;
    std::string message_;
};

}  // namespace protocomm
