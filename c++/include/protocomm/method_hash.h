#pragma once

#include <cstdint>

namespace protocomm {

constexpr uint32_t Fnv1a32(const char* s) noexcept {
    uint32_t hash = 0x811c9dc5u;
    while (*s) {
        hash ^= static_cast<uint32_t>(static_cast<unsigned char>(*s++));
        hash *= 0x01000193u;
    }
    return hash;
}

}  // namespace protocomm
