#pragma once

#include <string>

#include <boost/asio/awaitable.hpp>
#include <boost/asio/ip/tcp.hpp>
#include <boost/asio/read.hpp>
#include <boost/asio/write.hpp>
#include <boost/asio/use_awaitable.hpp>

#include "protocomm/status.h"
#include "frame.h"

namespace protocomm {

class SyncTransport {
public:
    static Status WriteHandshake(boost::asio::ip::tcp::socket& socket,
                                 const std::string& header);
    static Status ReadAndVerifyHandshake(boost::asio::ip::tcp::socket& socket,
                                        const std::string& expected);

    static Status WriteFrame(boost::asio::ip::tcp::socket& socket,
                             const FrameHeader& header,
                             const std::string& payload);
    static Status ReadFrame(boost::asio::ip::tcp::socket& socket,
                            FrameHeader& header,
                            std::string& payload);
};

class AsyncTransport {
public:
    static boost::asio::awaitable<Status>
    ReadHandshake(boost::asio::ip::tcp::socket& socket,
                  const std::string& expected);

    static boost::asio::awaitable<Status>
    ReadHandshakeRaw(boost::asio::ip::tcp::socket& socket,
                     size_t length, std::string& out);

    static boost::asio::awaitable<Status>
    WriteHandshake(boost::asio::ip::tcp::socket& socket,
                   const std::string& header);

    static boost::asio::awaitable<Status>
    WriteFrame(boost::asio::ip::tcp::socket& socket,
               const FrameHeader& header,
               const std::string& payload);

    static boost::asio::awaitable<Status>
    ReadFrame(boost::asio::ip::tcp::socket& socket,
              FrameHeader& header,
              std::string& payload);
};

}  // namespace protocomm
