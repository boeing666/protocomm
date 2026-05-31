#pragma once

namespace protocomm {

class Server;

class Service {
public:
    virtual ~Service() = default;
    virtual void RegisterWith(Server* server) = 0;
};

}  // namespace protocomm
