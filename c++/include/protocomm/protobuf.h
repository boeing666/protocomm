#pragma once

#include <string>
#include <string_view>

#include <google/protobuf/message_lite.h>

#include "protocomm/status.h"

namespace protocomm {

inline Status SerializeProto(const google::protobuf::MessageLite& msg,
                             std::string& out) {
    const size_t size = msg.ByteSizeLong();
    out.resize(size);
    if (size > 0 &&
        !msg.SerializeToArray(out.data(), static_cast<int>(size))) {
        return {StatusCode::INTERNAL, "protobuf serialization failed"};
    }
    return {};
}

inline Status ParseProto(std::string_view data,
                         google::protobuf::MessageLite& msg) {
    if (!msg.ParseFromArray(data.data(), static_cast<int>(data.size()))) {
        return {StatusCode::INTERNAL, "protobuf deserialization failed"};
    }
    return {};
}

}  // namespace protocomm
