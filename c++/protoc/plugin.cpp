#include "generator.hpp"
#include "generator_utils.hpp"

#include "protocomm/method_hash.h"

#include <google/protobuf/compiler/plugin.pb.h>
#ifdef _WIN32
#include <google/protobuf/io/io_win32.h>
#include <fcntl.h>
#else
#include <unistd.h>
#endif

#include <unordered_map>

using google::protobuf::compiler::CodeGeneratorRequest;
using google::protobuf::compiler::CodeGeneratorResponse;

int main() {
#ifdef _WIN32
    google::protobuf::io::win32::setmode(STDIN_FILENO, _O_BINARY);
    google::protobuf::io::win32::setmode(STDOUT_FILENO, _O_BINARY);
#endif

    CodeGeneratorRequest req;
    if (!req.ParseFromIstream(&std::cin)) {
        return 1;
    }

    CodeGeneratorResponse resp;

    {
        std::unordered_map<uint32_t, std::string> seen;
        for (const auto& file : req.proto_file()) {
            for (const auto& svc : file.service()) {
                for (const auto& m : svc.method()) {
                    const std::string full = protocomm::gen::method_full_name(
                        file.package(), svc.name(), m.name());
                    const uint32_t h = protocomm::Fnv1a32(full.c_str());
                    auto it = seen.find(h);
                    if (it != seen.end()) {
                        if (it->second == full) continue;
                        resp.set_error(
                            "protocomm: FNV1a32 method-ID collision between '"
                            + it->second + "' and '" + full + "'. "
                            "Rename one of the methods.");
                        resp.SerializeToOstream(&std::cout);
                        return 0;
                    }
                    seen.emplace(h, full);
                }
            }
        }
    }

    for (const auto& file : req.proto_file()) {
        if (file.service_size() == 0) {
            continue;
        }

        const std::string pkg = file.package();
        const std::string ns = protocomm::gen::ns_from_pkg(pkg);
        const bool has_ns = !ns.empty();

        for (const auto& svc : file.service()) {
            const std::string svc_name = svc.name();
            const std::string out_name = protocomm::gen::output_path(pkg, svc_name);

            auto* out = resp.add_file();
            out->set_name(out_name);

            protocomm::gen::ServiceContext ctx{
                file,
                svc,
                pkg,
                ns,
                svc_name,
                has_ns
            };

            std::string code;
            protocomm::gen::GenerateServiceFile(code, ctx);
            out->set_content(std::move(code));
        }
    }

    if (resp.file_size() == 0) {
        resp.set_error("No output files generated. Check that .proto contains services and methods.");
    }

    if (!resp.SerializeToOstream(&std::cout)) {
        return 1;
    }

    return 0;
}
