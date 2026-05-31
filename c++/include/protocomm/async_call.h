#pragma once

#include <chrono>
#include <future>
#include <optional>
#include <string>
#include <utility>

#include "protocomm/protobuf.h"
#include "protocomm/status.h"

namespace protocomm {

template <typename Response>
class AsyncUnaryCall {
public:
    using Result = std::pair<Status, Response>;

    explicit AsyncUnaryCall(std::future<std::pair<Status, std::string>> raw)
        : raw_(std::move(raw)) {}

    AsyncUnaryCall(AsyncUnaryCall&&) noexcept = default;
    AsyncUnaryCall& operator=(AsyncUnaryCall&&) noexcept = default;

    AsyncUnaryCall(const AsyncUnaryCall&) = delete;
    AsyncUnaryCall& operator=(const AsyncUnaryCall&) = delete;

    template <typename Rep, typename Period>
    AsyncUnaryCall& timeout(std::chrono::duration<Rep, Period> d) {
        deadline_ = std::chrono::steady_clock::now() + d;
        return *this;
    }

    Result get() {
        if (deadline_.has_value()) {
            if (raw_.wait_until(*deadline_) == std::future_status::timeout) {
                return {Status{StatusCode::DEADLINE_EXCEEDED, "timeout"},
                        Response{}};
            }
        }
        auto [st, bytes] = raw_.get();
        if (!st.ok()) return {st, Response{}};
        Response resp;
        Status ps = ParseProto(bytes, resp);
        if (!ps.ok()) return {ps, Response{}};
        return {Status{}, std::move(resp)};
    }

private:
    std::future<std::pair<Status, std::string>> raw_;
    std::optional<std::chrono::steady_clock::time_point> deadline_;
};

}  // namespace protocomm
