#pragma once

#include <google/protobuf/descriptor.pb.h>

#include <string>

namespace protocomm {
namespace gen {

struct ServiceContext {
    const google::protobuf::FileDescriptorProto& file;
    const google::protobuf::ServiceDescriptorProto& svc;
    std::string pkg;
    std::string ns;
    std::string svc_name;
    bool has_ns{false};
};

void GenerateServiceFile(std::string& code, const ServiceContext& ctx);

}  // namespace gen
}  // namespace protocomm
