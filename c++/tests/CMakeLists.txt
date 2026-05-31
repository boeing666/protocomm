# Use shared proto directory from parent, or fall back to local dir
if(DEFINED PROTOCOMM_PROTO_DIR AND EXISTS "${PROTOCOMM_PROTO_DIR}")
    set(PROTO_DIR "${PROTOCOMM_PROTO_DIR}")
else()
    set(PROTO_DIR "${CMAKE_CURRENT_SOURCE_DIR}")
endif()
set(GEN_DIR ${CMAKE_CURRENT_BINARY_DIR}/generated)

file(MAKE_DIRECTORY ${GEN_DIR})

set(CMAKE_CXX_STANDARD 20)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_CXX_EXTENSIONS OFF)

# ---------- find protoc executable ----------
if(TARGET protobuf::protoc)
    set(PROTOC_EXE $<TARGET_FILE:protobuf::protoc>)
elseif(TARGET protoc)
    set(PROTOC_EXE $<TARGET_FILE:protoc>)
else()
    find_program(PROTOC_EXE protoc REQUIRED)
endif()

# ---------- collect .proto files ----------
file(GLOB PROTO_FILES CONFIGURE_DEPENDS ${PROTO_DIR}/*.proto)

# ---------- generate sources for each .proto ----------
set(GEN_SRCS)

foreach(PROTO_FILE ${PROTO_FILES})
    get_filename_component(PROTO_NAME ${PROTO_FILE} NAME_WE)

    set(PB_CC  ${GEN_DIR}/${PROTO_NAME}.pb.cc)
    set(PB_H   ${GEN_DIR}/${PROTO_NAME}.pb.h)

    add_custom_command(
        OUTPUT  ${PB_CC} ${PB_H}
        COMMAND ${PROTOC_EXE}
                --proto_path=${PROTO_DIR}
                --cpp_out=${GEN_DIR}
                --protocomm_out=${GEN_DIR}
                --plugin=protoc-gen-protocomm=$<TARGET_FILE:protocomm_protoc_plugin>
                ${PROTO_FILE}
        DEPENDS ${PROTO_FILE} protocomm_protoc_plugin
        COMMENT "Generating protobuf + protocomm stubs for ${PROTO_NAME}.proto"
        VERBATIM
    )

    list(APPEND GEN_SRCS ${PB_CC})
endforeach()

# ---------- test executable ----------
file(GLOB_RECURSE TEST_SOURCES CONFIGURE_DEPENDS *.cpp)

add_executable(protocomm_tests
    ${TEST_SOURCES}
    ${GEN_SRCS}
)

set_target_properties(protocomm_tests PROPERTIES
    MSVC_RUNTIME_LIBRARY "MultiThreaded$<$<CONFIG:Debug>:Debug>"
)

target_link_libraries(protocomm_tests PRIVATE protocomm::protocomm)
target_include_directories(protocomm_tests PRIVATE ${GEN_DIR})

if(TARGET protobuf::libprotobuf)
    target_link_libraries(protocomm_tests PRIVATE protobuf::libprotobuf)
elseif(TARGET libprotobuf)
    target_link_libraries(protocomm_tests PRIVATE libprotobuf)
endif()

add_test(NAME protocomm_tests COMMAND protocomm_tests)
