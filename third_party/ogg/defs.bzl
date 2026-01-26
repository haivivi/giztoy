"""Ogg library repository rule."""

_OGG_VERSION = "1.3.6"

def _ogg_repo_impl(ctx):
    """Download and setup libogg source."""
    ctx.download_and_extract(
        url = "https://downloads.xiph.org/releases/ogg/libogg-{}.tar.gz".format(_OGG_VERSION),
        sha256 = "83e6704730683d004d20e21b8f7f55dcb3383cdf84c0daedf30bde175f774638",
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

ogg_repo = repository_rule(
    implementation = _ogg_repo_impl,
)
