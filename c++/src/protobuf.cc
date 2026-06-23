#include "protocomm/protobuf.h"

#include <cstddef>

#include <google/protobuf/message.h>
#include <google/protobuf/text_format.h>

namespace protocomm {

namespace {
constexpr std::size_t kBodyMax = 1024;
}

std::string RenderProto(const google::protobuf::Message& msg) {
    google::protobuf::TextFormat::Printer printer;
    printer.SetSingleLineMode(true);
    std::string out;
    if (!printer.PrintToString(msg, &out)) {
        return {};
    }
    if (out.size() > kBodyMax) {
        out.resize(kBodyMax);
        out += "\xE2\x80\xA6";
    }
    return out;
}

}
