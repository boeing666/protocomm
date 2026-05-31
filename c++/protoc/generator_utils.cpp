#include "generator_utils.hpp"

namespace protocomm {
namespace gen {

std::string ns_from_pkg(const std::string& pkg) {
    std::string out;
    for (char c : pkg) {
        out += (c == '.') ? "::" : std::string(1, c);
    }
    return out;
}

std::string service_full_name(const std::string& pkg, const std::string& svc) {
    if (pkg.empty()) {
        return svc;
    }
    return pkg + "." + svc;
}

std::string method_full_name(const std::string& pkg,
                             const std::string& svc,
                             const std::string& method) {
    return "/" + service_full_name(pkg, svc) + "/" + method;
}

std::string output_path(const std::string& pkg, const std::string& svc) {
    if (pkg.empty()) {
        return "protocomm/" + svc + ".h";
    }
    return "protocomm/" + pkg + "/" + svc + ".h";
}

std::string proto_type_to_cpp(std::string type) {
    if (!type.empty() && type[0] == '.') {
        type.erase(0, 1);
    }
    for (auto& c : type) {
        if (c == '.') {
            c = ':';
        }
    }
    std::string result;
    for (size_t i = 0; i < type.size(); ++i) {
        if (type[i] == ':') {
            result += "::";
        } else {
            result += type[i];
        }
    }
    return result;
}

} // namespace gen
} // namespace protocomm
