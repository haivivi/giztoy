"""Module extensions for external tools."""

load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")

# yq version
_YQ_VERSION = "4.44.3"

# yq checksums for different platforms (v4.44.3)
# Get latest from: https://github.com/mikefarah/yq/releases
# To update: shasum -a 256 yq_<os>_<arch>.tar.gz
_YQ_SHA256 = {
    "darwin_amd64": "",  # Will be verified on first download
    "darwin_arm64": "e53e12787e597e81f485a024d28e70dbe09e90e01ea08da060d8b0bc61f7fd38",
    "linux_amd64": "",
    "linux_arm64": "",
}

def _yq_repo_impl(ctx):
    """Implementation of yq repository rule."""
    os = ctx.os.name
    arch = ctx.os.arch

    # Map OS names
    if os == "mac os x" or os.startswith("darwin"):
        os_name = "darwin"
    elif os.startswith("linux"):
        os_name = "linux"
    else:
        fail("Unsupported OS: " + os)

    # Map architecture
    if arch == "amd64" or arch == "x86_64":
        arch_name = "amd64"
    elif arch == "aarch64" or arch == "arm64":
        arch_name = "arm64"
    else:
        fail("Unsupported architecture: " + arch)

    platform = "{}_{}".format(os_name, arch_name)
    
    # Download yq
    url = "https://github.com/mikefarah/yq/releases/download/v{}/yq_{}_{}.tar.gz".format(
        _YQ_VERSION, os_name, arch_name
    )
    
    ctx.download_and_extract(
        url = url,
        sha256 = _YQ_SHA256.get(platform, ""),
        stripPrefix = "",
    )
    
    # Create symlink for easy access (yq -> yq_darwin_arm64)
    ctx.symlink("yq_{}_{}".format(os_name, arch_name), "yq")
    
    # Create BUILD file
    ctx.file("BUILD.bazel", """
package(default_visibility = ["//visibility:public"])

exports_files(["yq_{os}_{arch}", "yq"])

# Use exports_files above; filegroup would conflict with the yq symlink name
""".format(os = os_name, arch = arch_name))

_yq_repo = repository_rule(
    implementation = _yq_repo_impl,
    attrs = {},
)

def _yq_extension_impl(module_ctx):
    _yq_repo(name = "yq")

yq = module_extension(
    implementation = _yq_extension_impl,
)

# =============================================================================
# Audio Libraries Extension (soxr, portaudio, opus)
# =============================================================================

_SOXR_VERSION = "0.1.3"
_PORTAUDIO_VERSION = "v19.7.0"
_OPUS_VERSION = "1.5.2"
_LAME_VERSION = "3.100"
_OGG_VERSION = "1.3.6"

def _soxr_repo_impl(ctx):
    """Download and setup soxr source."""
    ctx.download_and_extract(
        url = "https://sourceforge.net/projects/soxr/files/soxr-{}-Source.tar.xz".format(_SOXR_VERSION),
        stripPrefix = "soxr-{}-Source".format(_SOXR_VERSION),
    )
    ctx.file("BUILD.bazel", """
package(default_visibility = ["//visibility:public"])

filegroup(
    name = "soxr_c_srcs",
    srcs = glob(["src/*.c"], exclude = [
        "src/avfft*.c",
        "src/pffft*.c",
        "src/cr-core.c",      # Template included by cr32.c, cr64.c, etc.
        "src/vr-core.c",      # Template included by vr32.c
        "src/fft4g-core.c",   # Template included by fft4g32.c, fft4g64.c
        "src/util-core.c",    # Template included by util32.c, util64.c
        "src/util-simd.c",    # SIMD helper, not needed without SIMD
        "src/*32s.c",         # SIMD variants (not needed)
        "src/*64s.c",         # SIMD variants (not needed)
    ]),
)

filegroup(
    name = "soxr_headers",
    srcs = glob(["src/*.h"]),
)

filegroup(
    name = "soxr_templates",
    srcs = glob(["src/*.c"]),
)
""")

_soxr_repo = repository_rule(
    implementation = _soxr_repo_impl,
)

def _portaudio_repo_impl(ctx):
    """Download and setup portaudio source."""
    ctx.download_and_extract(
        url = "https://github.com/PortAudio/portaudio/archive/refs/tags/{}.tar.gz".format(_PORTAUDIO_VERSION),
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

_portaudio_repo = repository_rule(
    implementation = _portaudio_repo_impl,
)

def _opus_repo_impl(ctx):
    """Download and setup opus source."""
    ctx.download_and_extract(
        url = "https://downloads.xiph.org/releases/opus/opus-{}.tar.gz".format(_OPUS_VERSION),
        stripPrefix = "opus-{}".format(_OPUS_VERSION),
    )
    ctx.file("BUILD.bazel", """
package(default_visibility = ["//visibility:public"])

filegroup(
    name = "opus_public_headers",
    srcs = glob(["include/*.h"]),
)

filegroup(
    name = "opus_srcs",
    srcs = glob([
        "src/*.c",
        "celt/*.c",
        "silk/*.c",
        "silk/float/*.c",
    ], exclude = [
        "src/opus_demo.c",
        "src/opus_compare.c",
        "src/repacketizer_demo.c",
        "celt/opus_custom_demo.c",
    ]),
)

filegroup(
    name = "opus_internal_headers",
    srcs = glob([
        "src/*.h",
        "celt/*.h",
        "silk/*.h",
        "silk/float/*.h",
    ]),
)
""")

_opus_repo = repository_rule(
    implementation = _opus_repo_impl,
)

def _lame_repo_impl(ctx):
    """Download and setup LAME source."""
    ctx.download_and_extract(
        url = "https://sourceforge.net/projects/lame/files/lame/{version}/lame-{version}.tar.gz".format(version = _LAME_VERSION),
        stripPrefix = "lame-{}".format(_LAME_VERSION),
    )
    ctx.file("BUILD.bazel", """
package(default_visibility = ["//visibility:public"])

filegroup(
    name = "lame_srcs",
    srcs = glob([
        "libmp3lame/*.c",
        "libmp3lame/vector/*.c",
    ]),
)

filegroup(
    name = "lame_headers",
    srcs = glob([
        "include/*.h",
        "libmp3lame/*.h",
        "libmp3lame/vector/*.h",
    ]),
)

filegroup(
    name = "mpglib_srcs",
    srcs = glob(["mpglib/*.c"]),
)

filegroup(
    name = "mpglib_headers",
    srcs = glob(["mpglib/*.h"]),
)
""")

_lame_repo = repository_rule(
    implementation = _lame_repo_impl,
)

def _ogg_repo_impl(ctx):
    """Download and setup libogg source."""
    ctx.download_and_extract(
        url = "https://downloads.xiph.org/releases/ogg/libogg-{}.tar.gz".format(_OGG_VERSION),
        stripPrefix = "libogg-{}".format(_OGG_VERSION),
    )
    ctx.file("BUILD.bazel", """
package(default_visibility = ["//visibility:public"])

filegroup(
    name = "ogg_srcs",
    srcs = [
        "src/bitwise.c",
        "src/framing.c",
    ],
)

filegroup(
    name = "ogg_internal_headers",
    srcs = glob(["src/*.h"]),
)

filegroup(
    name = "ogg_headers",
    srcs = glob(["include/ogg/*.h"]),
)

exports_files(glob(["include/ogg/*.h"]))
exports_files(glob(["src/*.c"]))
exports_files(glob(["src/*.h"]))
""")

_ogg_repo = repository_rule(
    implementation = _ogg_repo_impl,
)

def _audio_libs_impl(ctx):
    """Module extension for audio libraries."""
    _soxr_repo(name = "soxr")
    _portaudio_repo(name = "portaudio")
    _opus_repo(name = "opus")
    _lame_repo(name = "lame")
    _ogg_repo(name = "ogg")

audio_libs = module_extension(
    implementation = _audio_libs_impl,
)
