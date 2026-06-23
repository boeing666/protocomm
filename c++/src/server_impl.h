#pragma once

#include "protocomm/server.h"
#include "session.h"

#include <atomic>
#include <mutex>
#include <thread>
#include <unordered_map>
#include <vector>

#include <boost/asio/co_spawn.hpp>
#include <boost/asio/detached.hpp>
#include <boost/asio/executor_work_guard.hpp>
#include <boost/asio/io_context.hpp>
#include <boost/asio/ip/tcp.hpp>
#include <boost/asio/use_awaitable.hpp>

namespace protocomm {

struct Server::Impl {
    boost::asio::io_context io_context;
    boost::asio::executor_work_guard<boost::asio::io_context::executor_type>
        work_guard{io_context.get_executor()};

    std::string bind_address;
    uint16_t bind_port = 0;
    std::string handshake_header;
    int io_thread_count = 1;

    Session::HandlerMap handlers;
    std::mutex handler_mu;

    Server::OnConnectCallback on_connect;
    Server::OnDisconnectCallback on_disconnect;
    Server::OnHandshakeCallback on_handshake;
    Server::Interceptor interceptor;

    std::mutex sessions_mu;
    std::unordered_map<uint64_t, std::shared_ptr<Session>> sessions;
    std::atomic<uint64_t> next_connection_id{1};

    std::vector<std::thread> threads;
    bool running = false;

    boost::asio::awaitable<void> AcceptLoop() {
        auto executor = co_await boost::asio::this_coro::executor;
        boost::asio::ip::tcp::acceptor acceptor(
            executor,
            {boost::asio::ip::make_address(bind_address), bind_port});

        acceptor.set_option(boost::asio::ip::tcp::acceptor::reuse_address(true));

        for (;;) {
            try {
                auto socket = co_await acceptor.async_accept(
                    boost::asio::use_awaitable);

                boost::system::error_code ec;
                socket.set_option(boost::asio::ip::tcp::no_delay(true), ec);

                uint64_t conn_id = next_connection_id.fetch_add(1);

                auto session = std::make_shared<Session>(
                    std::move(socket), handlers, handshake_header,
                    conn_id, on_connect, on_disconnect, on_handshake, interceptor);

                {
                    std::lock_guard lock(sessions_mu);
                    sessions[conn_id] = session;
                }

                auto wrapped = [this, session, conn_id]()
                    -> boost::asio::awaitable<void> {
                    co_await session->Run();
                    std::lock_guard lock(sessions_mu);
                    sessions.erase(conn_id);
                };

                boost::asio::co_spawn(executor,
                    wrapped(), boost::asio::detached);
            } catch (const boost::system::system_error&) {
                break;
            }
        }
    }

    Status Start() {
        running = true;
        boost::asio::co_spawn(io_context, AcceptLoop(), boost::asio::detached);

        for (int i = 0; i < io_thread_count; ++i) {
            threads.emplace_back([this]() { io_context.run(); });
        }

        return {};
    }
};

}  // namespace protocomm
