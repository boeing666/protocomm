#pragma once

#include <future>
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

    Result get() {
        auto [st, bytes] = raw_.get();
        if (!st.ok()) return {st, Response{}};
        Response resp;
        Status ps = ParseProto(bytes, resp);
        if (!ps.ok()) return {ps, Response{}};
        return {Status{}, std::move(resp)};
    }

private:
    std::future<std::pair<Status, std::string>> raw_;
};

}  // namespace protocomm
