"""Opus library repository rule."""

_OPUS_VERSION = "1.5.2"

def _opus_repo_impl(ctx):
    """Download and setup opus source."""
    ctx.download_and_extract(
        url = "https://downloads.xiph.org/releases/opus/opus-{}.tar.gz".format(_OPUS_VERSION),
        sha256 = "65c1d2f78b9f2fb20082c38cbe47c951ad5839345876e46941612ee87f9a7ce1",
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

opus_repo = repository_rule(
    implementation = _opus_repo_impl,
)
