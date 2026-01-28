# LED Strip Flash Example - Zig Version
# For more information about build system see
# https://docs.espressif.com/projects/esp-idf/en/latest/api-guides/build-system.html

cmake_minimum_required(VERSION 3.16)

# Include shared Zig installation cmake
include(${CMAKE_CURRENT_SOURCE_DIR}/../cmake/zig_install.cmake)

include($ENV{IDF_PATH}/tools/cmake/project.cmake)
project(led_strip_flash)
