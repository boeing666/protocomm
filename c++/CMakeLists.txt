cmake_minimum_required(VERSION 3.21)
project(protocomm LANGUAGES CXX)

set(CMAKE_CXX_STANDARD 20)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_CXX_EXTENSIONS OFF)

cmake_policy(SET CMP0167 NEW) # Policy CMP0167
cmake_policy(SET CMP0135 NEW) # Policy CMP0135

option(PROTOCOMM_BUILD_TESTS "Build tests" ON)

# When built standalone (not from root superbuild), try to find shared proto dir
if(NOT DEFINED PROTOCOMM_PROTO_DIR)
    if(EXISTS "${CMAKE_CURRENT_SOURCE_DIR}/../proto")
        set(PROTOCOMM_PROTO_DIR "${CMAKE_CURRENT_SOURCE_DIR}/../proto")
    endif()
endif()

include(${CMAKE_CURRENT_SOURCE_DIR}/cmake/boost.cmake)
include(${CMAKE_CURRENT_SOURCE_DIR}/cmake/protobuf.cmake)

add_subdirectory(src)
add_subdirectory(protoc)

if(PROTOCOMM_BUILD_TESTS)
    enable_testing()
    add_subdirectory(tests)
endif()
