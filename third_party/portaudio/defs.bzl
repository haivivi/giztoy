"""PortAudio library repository rule."""

_PORTAUDIO_VERSION = "v19.7.0"

def _portaudio_repo_impl(ctx):
    """Download and setup portaudio source."""
    ctx.download_and_extract(
        url = "https://github.com/PortAudio/portaudio/archive/refs/tags/{}.tar.gz".format(_PORTAUDIO_VERSION),
        sha256 = "5af29ba58bbdbb7bbcefaaecc77ec8fc413f0db6f4c4e286c40c3e1b83174fa0",
        stripPrefix = "portaudio-{}".format(_PORTAUDIO_VERSION.lstrip("v")),
    )
    ctx.file("BUILD.bazel", """
package(default_visibility = ["//visibility:public"])

filegroup(
    name = "portaudio_headers",
    srcs = glob(["include/*.h"]),
)

# Common sources for all platforms
_COMMON_SRCS = glob([
    "src/common/*.c",
])

# macOS-specific sources
filegroup(
    name = "portaudio_macos_srcs",
    srcs = _COMMON_SRCS + glob([
        "src/os/unix/*.c",
        "src/hostapi/coreaudio/*.c",
    ]),
)

filegroup(
    name = "portaudio_macos_internal_headers",
    srcs = glob([
        "src/common/*.h",
        "src/os/unix/*.h",
        "src/hostapi/coreaudio/*.h",
    ]),
)

# Linux-specific sources (ALSA)
filegroup(
    name = "portaudio_linux_srcs",
    srcs = _COMMON_SRCS + glob([
        "src/os/unix/*.c",
        "src/hostapi/alsa/*.c",
    ], allow_empty = True),
)

filegroup(
    name = "portaudio_linux_internal_headers",
    srcs = glob([
        "src/common/*.h",
        "src/os/unix/*.h",
        "src/hostapi/alsa/*.h",
    ], allow_empty = True),
)
""")

portaudio_repo = repository_rule(
    implementation = _portaudio_repo_impl,
)
