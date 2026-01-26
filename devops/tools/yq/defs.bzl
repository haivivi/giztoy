"""yq repository rule - YAML/JSON processor."""

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
        _YQ_VERSION,
        os_name,
        arch_name,
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

yq_repo = repository_rule(
    implementation = _yq_repo_impl,
    attrs = {},
)
