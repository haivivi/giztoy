# LED Strip Flash - Zig Version

idf_component_register(
    SRCS "src/main.c"
    INCLUDE_DIRS "."
    REQUIRES driver led_strip
)

esp_zig_build(
    FORCE_LINK
        led_strip_refresh
        led_strip_new_rmt_device
)
