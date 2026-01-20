# PortAudio source files and headers for Bazel

filegroup(
    name = "portaudio_headers",
    srcs = glob(["include/*.h"]),
    visibility = ["//visibility:public"],
)

# Common source files (platform independent)
filegroup(
    name = "portaudio_common_srcs",
    srcs = glob(["src/common/*.c"]),
    visibility = ["//visibility:public"],
)

# macOS CoreAudio host API
filegroup(
    name = "portaudio_coreaudio_srcs",
    srcs = glob(["src/hostapi/coreaudio/*.c"]),
    visibility = ["//visibility:public"],
)

# Linux ALSA host API
filegroup(
    name = "portaudio_alsa_srcs",
    srcs = glob(["src/hostapi/alsa/*.c"]),
    visibility = ["//visibility:public"],
)

# Unix OS-specific code (used by macOS and Linux)
filegroup(
    name = "portaudio_unix_srcs",
    srcs = glob(["src/os/unix/*.c"]),
    visibility = ["//visibility:public"],
)

# macOS source files
filegroup(
    name = "portaudio_macos_srcs",
    srcs = [
        ":portaudio_common_srcs",
        ":portaudio_coreaudio_srcs",
        ":portaudio_unix_srcs",
    ],
    visibility = ["//visibility:public"],
)

# Linux source files
filegroup(
    name = "portaudio_linux_srcs",
    srcs = [
        ":portaudio_common_srcs",
        ":portaudio_alsa_srcs",
        ":portaudio_unix_srcs",
    ],
    visibility = ["//visibility:public"],
)

# Internal headers - macOS
filegroup(
    name = "portaudio_macos_internal_headers",
    srcs = glob([
        "src/common/*.h",
        "src/os/unix/*.h",
        "src/hostapi/coreaudio/*.h",
    ]),
    visibility = ["//visibility:public"],
)

# Internal headers - Linux
filegroup(
    name = "portaudio_linux_internal_headers",
    srcs = glob([
        "src/common/*.h",
        "src/os/unix/*.h",
        "src/hostapi/alsa/*.h",
    ], allow_empty = True),
    visibility = ["//visibility:public"],
)
