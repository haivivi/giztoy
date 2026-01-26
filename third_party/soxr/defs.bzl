"""SoXR library repository rule."""

_SOXR_VERSION = "0.1.3"

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

soxr_repo = repository_rule(
    implementation = _soxr_repo_impl,
)
