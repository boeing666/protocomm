cmake_minimum_required(VERSION 3.21)
project(protocomm_superbuild LANGUAGES CXX)

# ---------------------------------------------------------------
# Options
# ---------------------------------------------------------------
option(PROTOCOMM_BUILD_CPP   "Build C++ library, plugin, and tests" ON)
option(PROTOCOMM_BUILD_GO    "Build Go plugin and run Go codegen"   ON)
option(PROTOCOMM_BUILD_TESTS "Build tests for both languages"       ON)

# ---------------------------------------------------------------
# Shared paths
# ---------------------------------------------------------------
set(PROTOCOMM_PROTO_DIR "${CMAKE_CURRENT_SOURCE_DIR}/proto"
    CACHE PATH "Directory containing shared .proto files")

if(WIN32)
    set(EXE_SUFFIX ".exe")
else()
    set(EXE_SUFFIX "")
endif()

# ---------------------------------------------------------------
# Phase 1: C++ subproject
#   Builds: protobuf (FetchContent), protoc, libprotobuf, libprotoc,
#           protocomm_protoc_plugin, protocomm library, tests.
#   C++ codegen is handled inside c++/tests/CMakeLists.txt using
#   PROTOCOMM_PROTO_DIR set above.
# ---------------------------------------------------------------
if(PROTOCOMM_BUILD_CPP)
    add_subdirectory(c++ cpp_build)
endif()

# ---------------------------------------------------------------
# Phase 2: Go protoc plugin
# ---------------------------------------------------------------
if(PROTOCOMM_BUILD_GO)
    set(GO_DIR "${CMAKE_CURRENT_SOURCE_DIR}/go")
    set(GO_PLUGIN_SRC_DIR "${GO_DIR}/protoc-gen-protocomm-go")
    set(GO_PLUGIN_OUT_DIR "${CMAKE_CURRENT_BINARY_DIR}/go_plugin")
    set(GO_PLUGIN_BIN "${GO_PLUGIN_OUT_DIR}/protoc-gen-protocomm-go${EXE_SUFFIX}")

    file(MAKE_DIRECTORY "${GO_PLUGIN_OUT_DIR}")
    file(GLOB GO_PLUGIN_SOURCES CONFIGURE_DEPENDS "${GO_PLUGIN_SRC_DIR}/*.go")

    add_custom_command(
        OUTPUT "${GO_PLUGIN_BIN}"
        COMMAND go build -o "${GO_PLUGIN_BIN}" .
        WORKING_DIRECTORY "${GO_PLUGIN_SRC_DIR}"
        DEPENDS ${GO_PLUGIN_SOURCES}
                "${GO_PLUGIN_SRC_DIR}/go.mod"
                "${GO_PLUGIN_SRC_DIR}/go.sum"
        COMMENT "[Go] Building protoc-gen-protocomm-go"
        VERBATIM
    )

    add_custom_target(go_protoc_plugin ALL
        DEPENDS "${GO_PLUGIN_BIN}"
    )

    # -----------------------------------------------------------
    # Phase 3: Go proto code generation
    # -----------------------------------------------------------
    # Resolve protoc from C++ FetchContent build
    if(TARGET protobuf::protoc)
        set(PROTOC_FOR_GO $<TARGET_FILE:protobuf::protoc>)
    elseif(TARGET protoc)
        set(PROTOC_FOR_GO $<TARGET_FILE:protoc>)
    else()
        find_program(PROTOC_FOR_GO protoc REQUIRED)
    endif()

    # protoc-gen-go is a prerequisite (not built by CMake)
    find_program(PROTOC_GEN_GO protoc-gen-go)
    if(NOT PROTOC_GEN_GO)
        message(WARNING
            "protoc-gen-go not found on PATH. "
            "Install: go install google.golang.org/protobuf/cmd/protoc-gen-go@latest")
    endif()

    file(GLOB PROTO_FILES CONFIGURE_DEPENDS "${PROTOCOMM_PROTO_DIR}/*.proto")
    file(MAKE_DIRECTORY "${CMAKE_CURRENT_BINARY_DIR}/go_gen_stamps")

    set(GO_GEN_STAMPS)
    foreach(PROTO_FILE ${PROTO_FILES})
        get_filename_component(PROTO_NAME ${PROTO_FILE} NAME_WE)

        set(STAMP "${CMAKE_CURRENT_BINARY_DIR}/go_gen_stamps/${PROTO_NAME}.stamp")

        # --go_opt=module=protocomm strips "protocomm/" prefix from go_package,
        # placing generated files into go/proto/example/, go/proto/math/, etc.
        add_custom_command(
            OUTPUT "${STAMP}"
            COMMAND ${PROTOC_FOR_GO}
                --proto_path=${PROTOCOMM_PROTO_DIR}
                --go_out=${GO_DIR}
                --go_opt=module=protocomm
                --plugin=protoc-gen-protocomm-go=${GO_PLUGIN_BIN}
                --protocomm-go_out=${GO_DIR}
                --protocomm-go_opt=module=protocomm
                ${PROTO_FILE}
            COMMAND ${CMAKE_COMMAND} -E touch "${STAMP}"
            DEPENDS ${PROTO_FILE} "${GO_PLUGIN_BIN}"
            COMMENT "[Go] Generating protobuf + protocomm stubs for ${PROTO_NAME}.proto"
            VERBATIM
        )
        list(APPEND GO_GEN_STAMPS "${STAMP}")
    endforeach()

    add_custom_target(go_proto_gen ALL
        DEPENDS ${GO_GEN_STAMPS}
    )
    add_dependencies(go_proto_gen go_protoc_plugin)

    # Ensure protoc is built before Go codegen
    if(TARGET protobuf::protoc)
        add_dependencies(go_proto_gen protobuf::protoc)
    elseif(TARGET protoc)
        add_dependencies(go_proto_gen protoc)
    endif()

    # -----------------------------------------------------------
    # Phase 4: Go build and test
    # -----------------------------------------------------------
    if(PROTOCOMM_BUILD_TESTS)
        add_custom_target(go_build
            COMMAND go build ./...
            WORKING_DIRECTORY "${GO_DIR}"
            COMMENT "[Go] Building Go module"
            VERBATIM
        )
        add_dependencies(go_build go_proto_gen)

        add_custom_target(go_test
            COMMAND go test -v -count=1 ./...
            WORKING_DIRECTORY "${GO_DIR}"
            COMMENT "[Go] Running Go tests"
            VERBATIM
        )
        add_dependencies(go_test go_build)
    endif()
endif()
