"""LAME MP3 library repository rule."""

_LAME_VERSION = "3.100"

def _lame_repo_impl(ctx):
    """Download and setup LAME source."""
    ctx.download_and_extract(
        url = "https://sourceforge.net/projects/lame/files/lame/{version}/lame-{version}.tar.gz".format(version = _LAME_VERSION),
        sha256 = "ddfe36cab873794038ae2c1210557ad34857a4b6bdc515785d1da9e175b1da1e",
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

lame_repo = repository_rule(
    implementation = _lame_repo_impl,
)
