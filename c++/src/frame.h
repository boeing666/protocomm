#pragma once

#include <cstdint>
#include <cstring>
#include <string>

namespace protocomm {

enum class FrameType : uint8_t {
    kRequest = 1,
    kResponse = 2,
    kPush = 3,
};

struct FrameHeader {
    static constexpr size_t kSize = 16;

    FrameType type = FrameType::kRequest;
    uint16_t status_code = 0;
    uint32_t call_id = 0;
    uint32_t method_id = 0;
    uint32_t payload_size = 0;

    void Serialize(uint8_t out[kSize]) const noexcept {
        out[0] = static_cast<uint8_t>(type);
        out[1] = 0;
        out[2] = static_cast<uint8_t>((status_code >> 8) & 0xFF);
        out[3] = static_cast<uint8_t>(status_code & 0xFF);
        WriteBE32(out + 4, call_id);
        WriteBE32(out + 8, method_id);
        WriteBE32(out + 12, payload_size);
    }

    static FrameHeader Deserialize(const uint8_t in[kSize]) noexcept {
        FrameHeader h;
        h.type = static_cast<FrameType>(in[0]);
        h.status_code = static_cast<uint16_t>((in[2] << 8) | in[3]);
        h.call_id = ReadBE32(in + 4);
        h.method_id = ReadBE32(in + 8);
        h.payload_size = ReadBE32(in + 12);
        return h;
    }

private:
    static void WriteBE32(uint8_t* out, uint32_t v) noexcept {
        out[0] = static_cast<uint8_t>((v >> 24) & 0xFF);
        out[1] = static_cast<uint8_t>((v >> 16) & 0xFF);
        out[2] = static_cast<uint8_t>((v >> 8) & 0xFF);
        out[3] = static_cast<uint8_t>(v & 0xFF);
    }

    static uint32_t ReadBE32(const uint8_t* in) noexcept {
        return (static_cast<uint32_t>(in[0]) << 24) |
               (static_cast<uint32_t>(in[1]) << 16) |
               (static_cast<uint32_t>(in[2]) << 8) |
               static_cast<uint32_t>(in[3]);
    }
};

static_assert(FrameHeader::kSize == 16);

}  // namespace protocomm
