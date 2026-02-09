"""ncnn static library built from source via CMake.

Downloads the ncnn source code and builds it with CMake to produce a
properly optimized static library with ARM NEON / x86 AVX support.

The pre-built releases lack ARM NEON optimizations on macOS, resulting
in ~1000x slower inference. Building from source fixes this.

Source: https://github.com/Tencent/ncnn
"""

_NCNN_VERSION = "20260113"
_NCNN_SHA256 = "53696039ee8ba5c8db6446bdf12a576b8d7f7b0c33bb6749f94688bddf5a3d5c"

def _ncnn_repo_impl(ctx):
    """Download ncnn source and build with CMake."""

    # Download source.
    ctx.download_and_extract(
        url = "https://github.com/Tencent/ncnn/releases/download/{}/ncnn-{}-full-source.zip".format(
            _NCNN_VERSION, _NCNN_VERSION,
        ),
        sha256 = _NCNN_SHA256,
        type = "zip",
    )

    # Detect CPU count for parallel build.
    os_name = ctx.os.name.lower()
    if "mac" in os_name or "darwin" in os_name:
        result = ctx.execute(["sysctl", "-n", "hw.ncpu"])
        nproc = result.stdout.strip() if result.return_code == 0 else "4"
    else:
        result = ctx.execute(["nproc"])
        nproc = result.stdout.strip() if result.return_code == 0 else "4"

    # CMake configure.
    ctx.execute(["mkdir", "-p", "build"])
    cmake_result = ctx.execute(
        [
            "cmake", "..",
            "-DCMAKE_BUILD_TYPE=Release",
            "-DNCNN_VULKAN=OFF",
            "-DNCNN_BUILD_TOOLS=OFF",
            "-DNCNN_BUILD_EXAMPLES=OFF",
            "-DNCNN_BUILD_BENCHMARK=OFF",
            "-DNCNN_BUILD_TESTS=OFF",
            "-DNCNN_OPENMP=OFF",
            "-DNCNN_SIMPLEOMP=ON",
            "-DNCNN_C_API=ON",
            "-DNCNN_INSTALL_SDK=OFF",
        ],
        working_directory = "build",
        timeout = 120,
    )
    if cmake_result.return_code != 0:
        fail("ncnn CMake configure failed:\n" + cmake_result.stderr)

    # Build.
    build_result = ctx.execute(
        ["make", "-j{}".format(nproc)],
        working_directory = "build",
        timeout = 600,
    )
    if build_result.return_code != 0:
        fail("ncnn build failed:\n" + build_result.stderr)

    # Collect artifacts into standard layout: include/ncnn/*.h + lib/libncnn.a
    ctx.execute(["mkdir", "-p", "include/ncnn", "lib"])
    ctx.execute(["cp", "build/src/libncnn.a", "lib/libncnn.a"])

    # Copy generated headers.
    ctx.execute(["cp", "build/src/platform.h", "include/ncnn/"])
    ctx.execute(["cp", "build/src/ncnn_export.h", "include/ncnn/"])
    ctx.execute(["cp", "build/src/layer_type_enum.h", "include/ncnn/"])
    ctx.execute(["cp", "build/src/layer_shader_type_enum.h", "include/ncnn/"])

    # Copy source headers.
    headers = [
        "c_api.h", "net.h", "mat.h", "blob.h", "layer.h", "option.h",
        "allocator.h", "paramdict.h", "datareader.h", "modelbin.h",
        "cpu.h", "pipelinecache.h", "gpu.h", "command.h", "pipeline.h",
        "simplestl.h", "simpleocv.h", "simplemath.h", "simpleomp.h",
        "simplevk.h", "expression.h", "layer_type.h", "layer_shader_type.h",
        "benchmark.h", "vulkan_header_fix.h",
    ]
    for h in headers:
        ctx.execute(["cp", "src/" + h, "include/ncnn/" + h])

    # Determine linkopts.
    if "mac" in os_name or "darwin" in os_name:
        linkopts = '["-lc++"]'
    else:
        linkopts = '["-lstdc++", "-lpthread"]'

    # Create BUILD file.
    ctx.file("BUILD.bazel", """\
load("@rules_cc//cc:cc_import.bzl", "cc_import")
load("@rules_cc//cc:cc_library.bzl", "cc_library")

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
    linkopts = {linkopts},
)
""".format(linkopts = linkopts))

ncnn_repo = repository_rule(
    implementation = _ncnn_repo_impl,
)
