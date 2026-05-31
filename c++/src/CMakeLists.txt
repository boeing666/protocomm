file(GLOB_RECURSE PROTOCOMM_SOURCES CONFIGURE_DEPENDS *.cc *.cpp)

if(NOT PROTOCOMM_SOURCES)
    message(FATAL_ERROR "No source files found in src/. Add at least one .cc or .cpp.")
endif()

add_library(protocomm STATIC ${PROTOCOMM_SOURCES})
add_library(protocomm::protocomm ALIAS protocomm)

target_compile_features(protocomm PRIVATE cxx_std_20)
target_include_directories(protocomm PUBLIC ../include)

if(DEFINED PROTOCOMM_BOOST_FETCH_INCLUDES)
    target_include_directories(protocomm PRIVATE ${PROTOCOMM_BOOST_FETCH_INCLUDES})
endif()

target_compile_definitions(protocomm PUBLIC _WIN32_WINNT=0x0A00)

target_link_libraries(protocomm PUBLIC Boost::system Boost::asio)
target_link_libraries(protocomm PUBLIC protobuf::libprotobuf)

set_target_properties(protocomm PROPERTIES
        MSVC_RUNTIME_LIBRARY "MultiThreaded$<$<CONFIG:Debug>:Debug>"
)
