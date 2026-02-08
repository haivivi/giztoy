"""ncnn pre-built static library repository rule.

Downloads the ncnn pre-built package from Tencent's official GitHub releases
and exposes the static library (.a) and C API headers for CGo linking.

ncnn is a high-performance neural network inference framework optimized for
mobile/embedded platforms. We use it for speaker embedding inference.

Source: https://github.com/Tencent/ncnn
"""

_NCNN_VERSION = "20260113"

_NCNN_SHA256 = {
    "darwin": "40bd0854ac5de56f730e07619d116d14a14d873befa53c739f2dcec1d8c60cf8",
    "linux": "7eb010ddd393efd571b3462c2a62ecea55297cdb025cbe1438bcf52b742a8cfd",
}

_NCNN_ARCHIVE = {
    "darwin": "ncnn-{}-macos.zip".format(_NCNN_VERSION),
    "linux": "ncnn-{}-ubuntu-2404.zip".format(_NCNN_VERSION),
}

def _ncnn_repo_impl(ctx):
    """Download ncnn pre-built package and normalize directory structure."""
    os = ctx.os.name.lower()

    if os in ("mac os x", "darwin", "macos") or os.startswith("darwin"):
        os_name = "darwin"
    elif os in ("linux",) or os.startswith("linux"):
        os_name = "linux"
    else:
        fail("Unsupported OS: '{}'. Supported: darwin, linux".format(os))

    if os_name not in _NCNN_SHA256:
        fail("No ncnn package for platform: {}".format(os_name))

    archive = _NCNN_ARCHIVE[os_name]
    url = "https://github.com/Tencent/ncnn/releases/download/{}/{}".format(
        _NCNN_VERSION,
        archive,
    )

    ctx.download_and_extract(
        url = url,
        sha256 = _NCNN_SHA256[os_name],
        type = "zip",
    )

    # Normalize directory structure: macOS uses framework layout, Linux uses standard layout.
    # We create a unified structure: include/ncnn/*.h + lib/libncnn.a + lib/libomp.a
    if os_name == "darwin":
        # macOS: frameworks layout → standard layout
        ctx.execute(["mkdir", "-p", "include/ncnn", "lib"])
        ctx.execute(["sh", "-c", "cp ncnn.framework/Versions/A/Headers/ncnn/*.h include/ncnn/"])
        ctx.execute(["cp", "ncnn.framework/Versions/A/ncnn", "lib/libncnn.a"])
        # OpenMP static library (bundled with ncnn macOS release)
        ctx.execute(["cp", "openmp.framework/Versions/A/openmp", "lib/libomp.a"])
    else:
        # Linux: standard layout
        prefix = "ncnn-{}-ubuntu-2404".format(_NCNN_VERSION)
        ctx.execute(["mkdir", "-p", "include/ncnn", "lib"])
        ctx.execute(["sh", "-c", "cp {}/include/ncnn/*.h include/ncnn/".format(prefix)])
        ctx.execute(["cp", "{}/lib/libncnn.a".format(prefix), "lib/libncnn.a"])

    # Create BUILD file with cc_import for static linking.
    # ncnn depends on OpenMP for multi-threaded inference and C++ stdlib.
    _load_cc = 'load("@rules_cc//cc:cc_import.bzl", "cc_import")\nload("@rules_cc//cc:cc_library.bzl", "cc_library")\n'

    if os_name == "darwin":
        ctx.file("BUILD.bazel", _load_cc + """\
package(default_visibility = ["//visibility:public"])

cc_import(
    name = "ncnn_lib",
    hdrs = glob(["include/ncnn/*.h"]),
    static_library = "lib/libncnn.a",
    includes = ["include"],
)

cc_import(
    name = "omp_lib",
    static_library = "lib/libomp.a",
)

cc_library(
    name = "ncnn",
    deps = [":ncnn_lib", ":omp_lib"],
    linkopts = ["-lc++"],
)
""")
    else:
        ctx.file("BUILD.bazel", _load_cc + """\
package(default_visibility = ["//visibility:public"])

cc_import(
    name = "ncnn_lib",
    hdrs = glob(["include/ncnn/*.h"]),
    static_library = "lib/libncnn.a",
    includes = ["include"],
)

cc_library(
    name = "ncnn",
    deps = [":ncnn_lib"],
    linkopts = ["-lstdc++", "-lgomp", "-lpthread"],
)
""")

ncnn_repo = repository_rule(
    implementation = _ncnn_repo_impl,
)
