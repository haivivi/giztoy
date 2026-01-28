# LED Strip Flash - Zig HAL Version
# Build with: bazel build //embed/esp/led_strip_flash:app --@embed_zig//bazel/esp:board=korvo2_v3

# Include I2C helper for TCA9554 support (Korvo-2)
# Path is relative to Bazel's temp build directory structure:
#   $WORK/project/main/CMakeLists.txt (this file)
#   $WORK/lib/esp/src/sal/i2c.cmake (target)
include(${CMAKE_CURRENT_SOURCE_DIR}/../../lib/esp/src/sal/i2c.cmake)

# Board selection (default: devkit)
# Use: -DZIG_BOARD=korvo2 for Korvo-2 board
if(NOT DEFINED ZIG_BOARD)
    set(ZIG_BOARD "devkit")
endif()
message(STATUS "[led_strip_flash] Board: ${ZIG_BOARD}")

idf_component_register(
    SRCS "src/main.c" ${I2C_C_SOURCES}
    INCLUDE_DIRS "."
    REQUIRES driver led_strip esp_driver_i2c
)

esp_zig_build(
    FORCE_LINK
        led_strip_refresh
        led_strip_new_rmt_device
        ${I2C_FORCE_LINK}
    ZIG_OPTIONS
        -Dboard=${ZIG_BOARD}
)
