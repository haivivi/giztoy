# LED Strip Flash Example - Zig Version
# Build with: bazel build //embed/esp/led_strip_flash:app
#
# This CMakeLists.txt is used by Bazel's esp_idf_app rule.
# The zig_install.cmake is injected via cmake_modules attribute.

cmake_minimum_required(VERSION 3.16)

# zig_install.cmake is provided by Bazel's esp_idf_app rule via cmake_modules
# For manual builds, ensure ZIG_INSTALL_CMAKE is set to the cmake module path
if(DEFINED ENV{ZIG_INSTALL_CMAKE})
    include($ENV{ZIG_INSTALL_CMAKE})
endif()

include($ENV{IDF_PATH}/tools/cmake/project.cmake)
project(led_strip_flash)
