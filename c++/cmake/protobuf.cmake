include_guard(GLOBAL)

find_package(Protobuf CONFIG QUIET)

if(NOT Protobuf_FOUND)
  include(FetchContent)

  FetchContent_Declare(
    protobuf
    GIT_REPOSITORY https://github.com/protocolbuffers/protobuf.git
    GIT_TAG v29.6
  )

  set(protobuf_BUILD_TESTS OFF CACHE BOOL "" FORCE)
  set(protobuf_BUILD_SHARED_LIBS OFF CACHE BOOL "" FORCE)

  FetchContent_MakeAvailable(protobuf)
endif()
