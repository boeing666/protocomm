add_executable(protocomm_protoc_plugin
    plugin.cpp
    generator.cpp
    generator_utils.cpp
)

target_include_directories(protocomm_protoc_plugin PRIVATE
    ${CMAKE_CURRENT_SOURCE_DIR}/../include
)

set_target_properties(protocomm_protoc_plugin PROPERTIES
    OUTPUT_NAME protoc-gen-protocomm
    MSVC_RUNTIME_LIBRARY "MultiThreaded$<$<CONFIG:Debug>:Debug>"
)

if(TARGET protobuf::libprotoc)
    target_link_libraries(protocomm_protoc_plugin PRIVATE protobuf::libprotoc protobuf::libprotobuf)
elseif(TARGET libprotoc)
    target_link_libraries(protocomm_protoc_plugin PRIVATE libprotoc libprotobuf)
else()
    message(FATAL_ERROR "Protobuf compiler libraries not found for protoc plugin.")
endif()

install(TARGETS protocomm_protoc_plugin
    RUNTIME DESTINATION bin
)
