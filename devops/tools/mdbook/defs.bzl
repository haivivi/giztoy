"""mdBook repository rule for documentation generation."""

_MDBOOK_VERSION = "0.4.44"

# mdBook checksums for different platforms
# Get latest from: https://github.com/rust-lang/mdBook/releases
# To update: shasum -a 256 mdbook-v<version>-<platform>.tar.gz
_MDBOOK_SHA256 = {
    "darwin_amd64": "416cd7f2d83194259a103746c2f35aef87427d9e48541397695929162e9d0557",
    "darwin_arm64": "a7e203a9b131ba045d6e4aff27f1a817059af9fe8174d86d78f79153da2e2b61",
    "linux_amd64": "326973fddabd7ff501f140ab529f3ede5f3f83f1a66ecc7e20adfec78eb6fc2a",
    "linux_arm64": "0019dfc4b32d63c1392aa264aed2253c1e0c2fb09216f8e2cc269bbfb8bb49b5",
}

def _mdbook_repo_impl(ctx):
    """Implementation of mdbook repository rule."""
    os = ctx.os.name.lower()
    arch = ctx.os.arch

    # Map OS names (handle various representations)
    if os in ("mac os x", "darwin", "macos") or os.startswith("darwin"):
        os_name = "apple-darwin"
        os_key = "darwin"
    elif os in ("linux",) or os.startswith("linux"):
        os_name = "unknown-linux-gnu"
        os_key = "linux"
    else:
        fail("Unsupported OS: '{}'. Supported: darwin, linux".format(os))

    # Map architecture
    if arch == "amd64" or arch == "x86_64":
        arch_name = "x86_64"
        arch_key = "amd64"
    elif arch == "aarch64" or arch == "arm64":
        arch_name = "aarch64"
        arch_key = "arm64"
    else:
        fail("Unsupported architecture: " + arch)

    platform_key = "{}_{}".format(os_key, arch_key)
    platform = "{}-{}".format(arch_name, os_name)

    # Verify platform is supported
    if platform_key not in _MDBOOK_SHA256:
        fail("Unsupported platform: {} (no SHA256 checksum available)".format(platform_key))

    # Download mdbook
    url = "https://github.com/rust-lang/mdBook/releases/download/v{}/mdbook-v{}-{}.tar.gz".format(
        _MDBOOK_VERSION,
        _MDBOOK_VERSION,
        platform,
    )

    ctx.download_and_extract(
        url = url,
        sha256 = _MDBOOK_SHA256[platform_key],
        stripPrefix = "",
    )

    # Create BUILD file
    ctx.file("BUILD.bazel", """
package(default_visibility = ["//visibility:public"])

exports_files(["mdbook"])

sh_binary(
    name = "mdbook_bin",
    srcs = ["mdbook"],
)
""")

mdbook_repo = repository_rule(
    implementation = _mdbook_repo_impl,
    attrs = {},
)
