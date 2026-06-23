#pragma once

#include <cstddef>
#include <string>
#include <string_view>

#include <google/protobuf/message.h>
#include <google/protobuf/message_lite.h>
#include <google/protobuf/text_format.h>

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

inline std::string RenderProto(const google::protobuf::Message& msg) {
    constexpr std::size_t kBodyMax = 1024;
    google::protobuf::TextFormat::Printer printer;
    printer.SetSingleLineMode(true);
    std::string out;
    if (!printer.PrintToString(msg, &out)) {
        return {};
    }
    if (out.size() > kBodyMax) {
        out.resize(kBodyMax);
        out += "\xE2\x80\xA6";  // … (truncation marker)
    }
    return out;
}

}  // namespace protocomm
