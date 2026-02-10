"""ONNX Runtime pre-built C library.

Downloads the official pre-built ONNX Runtime C API library for the
current platform. No compilation needed — just download and link.

Source: https://github.com/microsoft/onnxruntime
"""

_ORT_VERSION = "1.24.1"

# Pre-built library URLs and SHA256 hashes from GitHub releases.
_ORT_PLATFORMS = {
    "mac os x_aarch64": {
        "url": "https://github.com/microsoft/onnxruntime/releases/download/v{v}/onnxruntime-osx-arm64-{v}.tgz",
        "sha256": "c2969315cd9ce0f5fa04f6b53ff72cb92f87f7dcf38e88cacfa40c8f983fbba9",
        "strip_prefix": "onnxruntime-osx-arm64-{v}",
        "versioned_lib": "lib/libonnxruntime.1.24.1.dylib",
        "lib": "lib/libonnxruntime.dylib",
        "linkopts": ["-lc++"],
    },
    "linux_x86_64": {
        "url": "https://github.com/microsoft/onnxruntime/releases/download/v{v}/onnxruntime-linux-x64-{v}.tgz",
        "sha256": "9142552248b735920f9390027e4512a2cacf8946a1ffcbe9071a5c210531026f",
        "strip_prefix": "onnxruntime-linux-x64-{v}",
        "versioned_lib": "lib/libonnxruntime.so.1.24.1",
        "lib": "lib/libonnxruntime.so",
        "linkopts": ["-lstdc++", "-lpthread", "-ldl"],
    },
    "linux_aarch64": {
        "url": "https://github.com/microsoft/onnxruntime/releases/download/v{v}/onnxruntime-linux-aarch64-{v}.tgz",
        "sha256": "0f56edd68f7602df790b68b874a46b115add037e88385c6c842bb763b39b9f89",
        "strip_prefix": "onnxruntime-linux-aarch64-{v}",
        "versioned_lib": "lib/libonnxruntime.so.1.24.1",
        "lib": "lib/libonnxruntime.so",
        "linkopts": ["-lstdc++", "-lpthread", "-ldl"],
    },
}

def _platform_key(ctx):
    """Determine platform key from OS and arch."""
    os_name = ctx.os.name.lower()
    arch = ctx.os.arch

    if "mac" in os_name or "darwin" in os_name:
        return "mac os x_aarch64"
    elif "linux" in os_name:
        if arch in ("amd64", "x86_64"):
            return "linux_x86_64"
        elif arch in ("aarch64", "arm64"):
            return "linux_aarch64"

    fail("Unsupported platform: {} {}".format(os_name, arch))

def _onnxruntime_repo_impl(ctx):
    """Download pre-built ONNX Runtime C library."""
    key = _platform_key(ctx)
    platform = _ORT_PLATFORMS[key]
    v = _ORT_VERSION

    url = platform["url"].format(v = v)
    sha256 = platform["sha256"]
    strip_prefix = platform["strip_prefix"].format(v = v)

    ctx.download_and_extract(
        url = url,
        sha256 = sha256,
        stripPrefix = strip_prefix,
    )

    lib_path = platform["lib"]
    versioned_lib = platform["versioned_lib"]
    is_macos = lib_path.endswith(".dylib")

    if is_macos:
        # Fix macOS dylib install_name for @rpath resolution.
        # The pre-built dylib has install_name @rpath/libonnxruntime.1.x.x.dylib.
        # We change it to @rpath/libonnxruntime.dylib so the binary's LC_RPATH
        # entries can locate it in Bazel's _solib directories.
        ctx.execute([
            "install_name_tool", "-id",
            "@rpath/libonnxruntime.dylib",
            versioned_lib,
        ])
        # Ensure the non-versioned dylib is a real file (not just symlink).
        ctx.execute(["rm", "-f", lib_path])
        ctx.execute(["cp", versioned_lib, lib_path])
    else:
        # Linux: replace symlink with real copy and fix soname.
        # The versioned lib has soname "libonnxruntime.so.1" but Bazel's
        # cc_import only copies "libonnxruntime.so". Fix soname to match.
        ctx.execute(["rm", "-f", lib_path])
        ctx.execute(["cp", versioned_lib, lib_path])
        # patchelf --set-soname if available (installed via apt on CI).
        ctx.execute(["patchelf", "--set-soname", "libonnxruntime.so", lib_path])

    linkopts = ", ".join(['"{}"'.format(l) for l in platform["linkopts"]])

    ctx.file("BUILD.bazel", """\
load("@rules_cc//cc:cc_import.bzl", "cc_import")
load("@rules_cc//cc:cc_library.bzl", "cc_library")

package(default_visibility = ["//visibility:public"])

cc_import(
    name = "onnxruntime_lib",
    hdrs = glob(["include/*.h"]),
    shared_library = "{lib_path}",
    includes = ["include"],
)

cc_library(
    name = "onnxruntime",
    deps = [":onnxruntime_lib"],
    linkopts = [{linkopts}],
)
""".format(
        lib_path = lib_path,
        linkopts = linkopts,
    ))

onnxruntime_repo = repository_rule(
    implementation = _onnxruntime_repo_impl,
)
