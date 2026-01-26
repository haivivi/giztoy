"""Luau scripting language repository rule."""

_LUAU_VERSION = "0.706"

def _luau_repo_impl(ctx):
    """Download and setup Luau source."""
    ctx.download_and_extract(
        url = "https://github.com/luau-lang/luau/archive/refs/tags/{}.tar.gz".format(_LUAU_VERSION),
        stripPrefix = "luau-{}".format(_LUAU_VERSION),
    )
    ctx.file("BUILD.bazel", """
package(default_visibility = ["//visibility:public"])

# Common sources
filegroup(
    name = "common_srcs",
    srcs = glob(["Common/src/*.cpp"]),
)

filegroup(
    name = "common_hdrs",
    srcs = glob(["Common/include/**/*.h"]),
)

# AST sources
filegroup(
    name = "ast_srcs",
    srcs = glob(["Ast/src/*.cpp"]),
)

filegroup(
    name = "ast_hdrs",
    srcs = glob(["Ast/include/**/*.h"]),
)

# Compiler sources
filegroup(
    name = "compiler_srcs",
    srcs = glob(["Compiler/src/*.cpp"]),
)

filegroup(
    name = "compiler_hdrs",
    srcs = glob(["Compiler/include/**/*.h"]) + glob(["Compiler/src/*.h"]),
)

# VM sources
filegroup(
    name = "vm_srcs",
    srcs = glob(["VM/src/*.cpp"]),
)

filegroup(
    name = "vm_hdrs",
    srcs = glob(["VM/include/*.h"]) + glob(["VM/src/*.h"]),
)

# CodeGen sources (optional JIT)
filegroup(
    name = "codegen_srcs",
    srcs = glob(["CodeGen/src/*.cpp"]),
)

filegroup(
    name = "codegen_hdrs",
    srcs = glob(["CodeGen/include/**/*.h"]) + glob(["CodeGen/src/*.h"]),
)

# All sources combined for static library build
filegroup(
    name = "all_srcs",
    srcs = [
        ":common_srcs",
        ":ast_srcs",
        ":compiler_srcs",
        ":vm_srcs",
    ],
)

filegroup(
    name = "all_hdrs",
    srcs = [
        ":common_hdrs",
        ":ast_hdrs",
        ":compiler_hdrs",
        ":vm_hdrs",
    ],
)

# Export all source files
exports_files(glob(["**/*.cpp"]) + glob(["**/*.h"]))
""")

luau_repo = repository_rule(
    implementation = _luau_repo_impl,
)
