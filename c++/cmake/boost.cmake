include_guard(GLOBAL)

set(BOOST_MIN_VERSION 1.78.0)

find_package(Boost ${BOOST_MIN_VERSION} QUIET COMPONENTS system)

set(PROTOCOMM_HAVE_SYSTEM_BOOST_BEAST OFF)
if(Boost_FOUND)
    foreach(_inc IN LISTS Boost_INCLUDE_DIRS)
        if(EXISTS "${_inc}/boost/beast.hpp")
            set(PROTOCOMM_HAVE_SYSTEM_BOOST_BEAST ON)
            break()
        endif()
    endforeach()
endif()

if(NOT PROTOCOMM_HAVE_SYSTEM_BOOST_BEAST)
    include(FetchContent)

    set(BOOST_DOWNLOAD_VERSION 1.90.0)
    set(BOOST_DOWNLOAD_NAME "boost-${BOOST_DOWNLOAD_VERSION}")

    FetchContent_Declare(
        boost
        URL "https://github.com/boostorg/boost/releases/download/${BOOST_DOWNLOAD_NAME}/${BOOST_DOWNLOAD_NAME}-cmake.tar.gz"
        DOWNLOAD_EXTRACT_TIMESTAMP TRUE
    )

    set(BOOST_INCLUDE_LIBRARIES "asio;align;throw_exception;system" CACHE STRING "" FORCE)
    set(BOOST_ENABLE_CMAKE ON CACHE BOOL "" FORCE)

    FetchContent_MakeAvailable(boost)
    FetchContent_GetProperties(boost)

    set(_boost_mods asio align)

    set(_incs "")
    foreach(m IN LISTS _boost_mods)
        message(STATUS "Checking for Boost module ${m} in ${boost_SOURCE_DIR}/libs/${m}/include")
        if(EXISTS "${boost_SOURCE_DIR}/libs/${m}/include")
            list(APPEND _incs "${boost_SOURCE_DIR}/libs/${m}/include")
        endif()
    endforeach()

    if(NOT _incs)
        message(FATAL_ERROR "Fetched Boost: cannot build include list under ${boost_SOURCE_DIR}/libs/*/include")
    endif()

    set(PROTOCOMM_BOOST_FETCH_INCLUDES "${_incs}" CACHE INTERNAL "")
endif()

if(PROTOCOMM_HAVE_SYSTEM_BOOST_BEAST AND NOT TARGET Boost::asio)
    add_library(Boost::asio INTERFACE IMPORTED)
    if(TARGET Boost::headers)
        target_link_libraries(Boost::asio INTERFACE Boost::headers)
    endif()
endif()
