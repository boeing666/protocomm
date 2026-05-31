#pragma once

#include <string>

namespace protocomm {
namespace gen {

std::string ns_from_pkg(const std::string& pkg);

std::string service_full_name(const std::string& pkg, const std::string& svc);

std::string method_full_name(const std::string& pkg,
                             const std::string& svc,
                             const std::string& method);

std::string output_path(const std::string& pkg, const std::string& svc);

std::string proto_type_to_cpp(std::string type);

} // namespace gen
} // namespace protocomm
