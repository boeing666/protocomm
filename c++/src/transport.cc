#include "transport.h"

#include <array>

#include <boost/asio/buffer.hpp>

namespace protocomm {

Status SyncTransport::WriteHandshake(boost::asio::ip::tcp::socket& socket,
                                     const std::string& header) {
    if (header.empty()) return {};
    boost::system::error_code ec;
    boost::asio::write(socket, boost::asio::buffer(header), ec);
    if (ec) return {StatusCode::UNAVAILABLE, "handshake write: " + ec.message()};
    return {};
}

Status SyncTransport::ReadAndVerifyHandshake(
    boost::asio::ip::tcp::socket& socket,
    const std::string& expected) {
    if (expected.empty()) return {};
    std::string buf(expected.size(), '\0');
    boost::system::error_code ec;
    boost::asio::read(socket, boost::asio::buffer(buf), ec);
    if (ec) return {StatusCode::UNAVAILABLE, "handshake read: " + ec.message()};
    if (buf != expected) {
        return {StatusCode::FAILED_PRECONDITION, "handshake mismatch"};
    }
    return {};
}

Status SyncTransport::WriteFrame(boost::asio::ip::tcp::socket& socket,
                                 const FrameHeader& header,
                                 const std::string& payload) {
    std::array<uint8_t, FrameHeader::kSize> buf{};
    header.Serialize(buf.data());

    boost::system::error_code ec;
    std::array<boost::asio::const_buffer, 2> bufs = {
        boost::asio::buffer(buf),
        boost::asio::buffer(payload),
    };
    boost::asio::write(socket, bufs, ec);
    if (ec) return {StatusCode::UNAVAILABLE, "write frame: " + ec.message()};
    return {};
}

Status SyncTransport::ReadFrame(boost::asio::ip::tcp::socket& socket,
                                FrameHeader& header,
                                std::string& payload) {
    std::array<uint8_t, FrameHeader::kSize> buf{};
    boost::system::error_code ec;
    boost::asio::read(socket, boost::asio::buffer(buf), ec);
    if (ec) return {StatusCode::UNAVAILABLE, "read header: " + ec.message()};

    header = FrameHeader::Deserialize(buf.data());

    if (header.payload_size > 0) {
        payload.resize(header.payload_size);
        boost::asio::read(socket, boost::asio::buffer(payload), ec);
        if (ec) return {StatusCode::UNAVAILABLE, "read payload: " + ec.message()};
    } else {
        payload.clear();
    }
    return {};
}

boost::asio::awaitable<Status>
AsyncTransport::ReadHandshake(boost::asio::ip::tcp::socket& socket,
                              const std::string& expected) {
    if (expected.empty()) co_return Status{};
    try {
        std::string buf(expected.size(), '\0');
        co_await boost::asio::async_read(
            socket, boost::asio::buffer(buf), boost::asio::use_awaitable);
        if (buf != expected) {
            co_return Status{StatusCode::FAILED_PRECONDITION, "handshake mismatch"};
        }
        co_return Status{};
    } catch (const boost::system::system_error& e) {
        co_return Status{StatusCode::UNAVAILABLE,
                         std::string("handshake read: ") + e.what()};
    }
}

boost::asio::awaitable<Status>
AsyncTransport::ReadHandshakeRaw(boost::asio::ip::tcp::socket& socket,
                                  size_t length, std::string& out) {
    try {
        out.resize(length);
        co_await boost::asio::async_read(
            socket, boost::asio::buffer(out), boost::asio::use_awaitable);
        co_return Status{};
    } catch (const boost::system::system_error& e) {
        co_return Status{StatusCode::UNAVAILABLE,
                         std::string("handshake read: ") + e.what()};
    }
}

boost::asio::awaitable<Status>
AsyncTransport::WriteHandshake(boost::asio::ip::tcp::socket& socket,
                               const std::string& header) {
    if (header.empty()) co_return Status{};
    try {
        co_await boost::asio::async_write(
            socket, boost::asio::buffer(header), boost::asio::use_awaitable);
        co_return Status{};
    } catch (const boost::system::system_error& e) {
        co_return Status{StatusCode::UNAVAILABLE,
                         std::string("handshake write: ") + e.what()};
    }
}

boost::asio::awaitable<Status>
AsyncTransport::WriteFrame(boost::asio::ip::tcp::socket& socket,
                           const FrameHeader& header,
                           const std::string& payload) {
    try {
        std::array<uint8_t, FrameHeader::kSize> hdr_buf{};
        header.Serialize(hdr_buf.data());

        std::array<boost::asio::const_buffer, 2> bufs = {
            boost::asio::buffer(hdr_buf),
            boost::asio::buffer(payload),
        };
        co_await boost::asio::async_write(
            socket, bufs, boost::asio::use_awaitable);
        co_return Status{};
    } catch (const boost::system::system_error& e) {
        co_return Status{StatusCode::UNAVAILABLE,
                         std::string("write frame: ") + e.what()};
    }
}

boost::asio::awaitable<Status>
AsyncTransport::ReadFrame(boost::asio::ip::tcp::socket& socket,
                          FrameHeader& header,
                          std::string& payload) {
    try {
        std::array<uint8_t, FrameHeader::kSize> buf{};
        co_await boost::asio::async_read(
            socket, boost::asio::buffer(buf), boost::asio::use_awaitable);

        header = FrameHeader::Deserialize(buf.data());

        if (header.payload_size > 0) {
            payload.resize(header.payload_size);
            co_await boost::asio::async_read(
                socket, boost::asio::buffer(payload),
                boost::asio::use_awaitable);
        } else {
            payload.clear();
        }
        co_return Status{};
    } catch (const boost::system::system_error& e) {
        co_return Status{StatusCode::UNAVAILABLE,
                         std::string("read frame: ") + e.what()};
    }
}

}  // namespace protocomm
