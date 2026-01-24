load("@gazelle//:deps.bzl", "go_repository")

"""Module extensions for external tools."""

load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")

# =============================================================================
# mdBook Extension (for documentation)
# =============================================================================

_MDBOOK_VERSION = "0.4.44"

# mdBook checksums for different platforms
# Get latest from: https://github.com/rust-lang/mdBook/releases
# To update: shasum -a 256 mdbook-v<version>-<platform>.tar.gz
_MDBOOK_SHA256 = {
    "darwin_amd64": "",
    "darwin_arm64": "a7e203a9b131ba045d6e4aff27f1a817059af9fe8174d86d78f79153da2e2b61",
    "linux_amd64": "",
    "linux_arm64": "",
}

def _mdbook_repo_impl(ctx):
    """Implementation of mdbook repository rule."""
    os = ctx.os.name
    arch = ctx.os.arch

    # Map OS names
    if os == "mac os x" or os.startswith("darwin"):
        os_name = "apple-darwin"
        os_key = "darwin"
    elif os.startswith("linux"):
        os_name = "unknown-linux-gnu"
        os_key = "linux"
    else:
        fail("Unsupported OS: " + os)

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

    # Download mdbook
    url = "https://github.com/rust-lang/mdBook/releases/download/v{}/mdbook-v{}-{}.tar.gz".format(
        _MDBOOK_VERSION,
        _MDBOOK_VERSION,
        platform,
    )

    ctx.download_and_extract(
        url = url,
        sha256 = _MDBOOK_SHA256.get(platform_key, ""),
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

_mdbook_repo = repository_rule(
    implementation = _mdbook_repo_impl,
    attrs = {},
)

def _mdbook_extension_impl(module_ctx):
    _mdbook_repo(name = "mdbook")

mdbook = module_extension(
    implementation = _mdbook_extension_impl,
)

# =============================================================================
# Mermaid.js Extension (for documentation diagrams)
# =============================================================================

_MERMAID_VERSION = "11.4.1"

def _mermaid_repo_impl(ctx):
    """Download mermaid.min.js from CDN."""
    ctx.download(
        url = "https://cdn.jsdelivr.net/npm/mermaid@{}/dist/mermaid.min.js".format(_MERMAID_VERSION),
        output = "mermaid.min.js",
    )

    ctx.file("BUILD.bazel", """
package(default_visibility = ["//visibility:public"])

exports_files(["mermaid.min.js"])
""")

_mermaid_repo = repository_rule(
    implementation = _mermaid_repo_impl,
    attrs = {},
)

def _mermaid_extension_impl(module_ctx):
    _mermaid_repo(name = "mermaid")

mermaid = module_extension(
    implementation = _mermaid_extension_impl,
)

# =============================================================================
# yq Extension
# =============================================================================

# yq version
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

_yq_repo = repository_rule(
    implementation = _yq_repo_impl,
    attrs = {},
)

def _yq_extension_impl(module_ctx):
    _yq_repo(name = "yq")

yq = module_extension(
    implementation = _yq_extension_impl,
)

# =============================================================================
# Audio Libraries Extension (soxr, portaudio, opus)
# =============================================================================

_SOXR_VERSION = "0.1.3"
_PORTAUDIO_VERSION = "v19.7.0"
_OPUS_VERSION = "1.5.2"
_LAME_VERSION = "3.100"
_OGG_VERSION = "1.3.6"

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

_soxr_repo = repository_rule(
    implementation = _soxr_repo_impl,
)

def _portaudio_repo_impl(ctx):
    """Download and setup portaudio source."""
    ctx.download_and_extract(
        url = "https://github.com/PortAudio/portaudio/archive/refs/tags/{}.tar.gz".format(_PORTAUDIO_VERSION),
        stripPrefix = "portaudio-{}".format(_PORTAUDIO_VERSION.lstrip("v")),
    )
    ctx.file("BUILD.bazel", """
package(default_visibility = ["//visibility:public"])

filegroup(
    name = "portaudio_headers",
    srcs = glob(["include/*.h"]),
)

# Common sources for all platforms
_COMMON_SRCS = glob([
    "src/common/*.c",
])

# macOS-specific sources
filegroup(
    name = "portaudio_macos_srcs",
    srcs = _COMMON_SRCS + glob([
        "src/os/unix/*.c",
        "src/hostapi/coreaudio/*.c",
    ]),
)

filegroup(
    name = "portaudio_macos_internal_headers",
    srcs = glob([
        "src/common/*.h",
        "src/os/unix/*.h",
        "src/hostapi/coreaudio/*.h",
    ]),
)

# Linux-specific sources (ALSA)
filegroup(
    name = "portaudio_linux_srcs",
    srcs = _COMMON_SRCS + glob([
        "src/os/unix/*.c",
        "src/hostapi/alsa/*.c",
    ], allow_empty = True),
)

filegroup(
    name = "portaudio_linux_internal_headers",
    srcs = glob([
        "src/common/*.h",
        "src/os/unix/*.h",
        "src/hostapi/alsa/*.h",
    ], allow_empty = True),
)
""")

_portaudio_repo = repository_rule(
    implementation = _portaudio_repo_impl,
)

def _opus_repo_impl(ctx):
    """Download and setup opus source."""
    ctx.download_and_extract(
        url = "https://downloads.xiph.org/releases/opus/opus-{}.tar.gz".format(_OPUS_VERSION),
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

_opus_repo = repository_rule(
    implementation = _opus_repo_impl,
)

def _lame_repo_impl(ctx):
    """Download and setup LAME source."""
    ctx.download_and_extract(
        url = "https://sourceforge.net/projects/lame/files/lame/{version}/lame-{version}.tar.gz".format(version = _LAME_VERSION),
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

_lame_repo = repository_rule(
    implementation = _lame_repo_impl,
)

def _ogg_repo_impl(ctx):
    """Download and setup libogg source."""
    ctx.download_and_extract(
        url = "https://downloads.xiph.org/releases/ogg/libogg-{}.tar.gz".format(_OGG_VERSION),
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

_ogg_repo = repository_rule(
    implementation = _ogg_repo_impl,
)

def _audio_libs_impl(ctx):
    """Module extension for audio libraries."""
    _soxr_repo(name = "soxr")
    _portaudio_repo(name = "portaudio")
    _opus_repo(name = "opus")
    _lame_repo(name = "lame")
    _ogg_repo(name = "ogg")

audio_libs = module_extension(
    implementation = _audio_libs_impl,
)

def go_dependencies():
    go_repository(
        name = "com_github_alicebob_gopher_json",
        importpath = "github.com/alicebob/gopher-json",
        sum = "h1:HbKu58rmZpUGpz5+4FfNmIU+FmZg2P3Xaj2v2bfNWmk=",
        version = "v0.0.0-20200520072559-a9ecdc9d1d3a",
    )
    go_repository(
        name = "com_github_alicebob_miniredis_v2",
        importpath = "github.com/alicebob/miniredis/v2",
        sum = "h1:+lwAJYjvvdIVg6doFHuotFjueJ/7KY10xo/vm3X3Scw=",
        version = "v2.23.0",
    )
    go_repository(
        name = "com_github_azure_azure_sdk_for_go_sdk_azcore",
        importpath = "github.com/Azure/azure-sdk-for-go/sdk/azcore",
        sum = "h1:g0EZJwz7xkXQiZAI5xi9f3WWFYBlX1CPTrR+NDToRkQ=",
        version = "v1.17.0",
    )
    go_repository(
        name = "com_github_azure_azure_sdk_for_go_sdk_azidentity",
        importpath = "github.com/Azure/azure-sdk-for-go/sdk/azidentity",
        sum = "h1:tfLQ34V6F7tVSwoTf/4lH5sE0o6eCJuNDTmH09nDpbc=",
        version = "v1.7.0",
    )
    go_repository(
        name = "com_github_azure_azure_sdk_for_go_sdk_internal",
        importpath = "github.com/Azure/azure-sdk-for-go/sdk/internal",
        sum = "h1:ywEEhmNahHBihViHepv3xPBn1663uRv2t2q/ESv9seY=",
        version = "v1.10.0",
    )
    go_repository(
        name = "com_github_azuread_microsoft_authentication_library_for_go",
        importpath = "github.com/AzureAD/microsoft-authentication-library-for-go",
        sum = "h1:XHOnouVk1mxXfQidrMEnLlPk9UMeRtyBTnEFtxkV0kU=",
        version = "v1.2.2",
    )
    go_repository(
        name = "com_github_beorn7_perks",
        importpath = "github.com/beorn7/perks",
        sum = "h1:VlbKKnNfV8bJzeqoa4cOKqO6bYr3WgKZxO8Z16+hsOM=",
        version = "v1.0.1",
    )
    go_repository(
        name = "com_github_cespare_xxhash_v2",
        importpath = "github.com/cespare/xxhash/v2",
        sum = "h1:UL815xU9SqsFlibzuggzjXhog7bL6oX9BbNZnL2UFvs=",
        version = "v2.3.0",
    )
    go_repository(
        name = "com_github_cncf_xds_go",
        importpath = "github.com/cncf/xds/go",
        sum = "h1:Y8xYupdHxryycyPlc9Y+bSQAYZnetRJ70VMVKm5CKI0=",
        version = "v0.0.0-20251022180443-0feb69152e9f",
    )
    go_repository(
        name = "com_github_cockroachdb_errors",
        importpath = "github.com/cockroachdb/errors",
        sum = "h1:xSEW75zKaKCWzR3OfxXUxgrk/NtT4G1MiOv5lWZazG8=",
        version = "v1.11.1",
    )
    go_repository(
        name = "com_github_cockroachdb_logtags",
        importpath = "github.com/cockroachdb/logtags",
        sum = "h1:r6VH0faHjZeQy818SGhaone5OnYfxFR/+AzdY3sf5aE=",
        version = "v0.0.0-20230118201751-21c54148d20b",
    )
    go_repository(
        name = "com_github_cockroachdb_pebble",
        importpath = "github.com/cockroachdb/pebble",
        sum = "h1:pcFh8CdCIt2kmEpK0OIatq67Ln9uGDYY3d5XnE0LJG4=",
        version = "v1.1.0",
    )
    go_repository(
        name = "com_github_cockroachdb_redact",
        importpath = "github.com/cockroachdb/redact",
        sum = "h1:u1PMllDkdFfPWaNGMyLD1+so+aq3uUItthCFqzwPJ30=",
        version = "v1.1.5",
    )
    go_repository(
        name = "com_github_cockroachdb_tokenbucket",
        importpath = "github.com/cockroachdb/tokenbucket",
        sum = "h1:zuQyyAKVxetITBuuhv3BI9cMrmStnpT18zmgmTxunpo=",
        version = "v0.0.0-20230807174530-cc333fc44b06",
    )
    go_repository(
        name = "com_github_cpuguy83_go_md2man_v2",
        importpath = "github.com/cpuguy83/go-md2man/v2",
        sum = "h1:XJtiaUW6dEEqVuZiMTn1ldk455QWwEIsMIJlo5vtkx0=",
        version = "v2.0.6",
    )
    go_repository(
        name = "com_github_datadog_zstd",
        importpath = "github.com/DataDog/zstd",
        sum = "h1:EndNeuB0l9syBZhut0wns3gV1hL8zX8LIu6ZiVHWLIQ=",
        version = "v1.4.5",
    )
    go_repository(
        name = "com_github_davecgh_go_spew",
        importpath = "github.com/davecgh/go-spew",
        sum = "h1:vj9j/u1bqnvCEfJOwUhtlOARqs3+rkHYY13jYWTU97c=",
        version = "v1.1.1",
    )
    go_repository(
        name = "com_github_dgraph_io_badger_v4",
        importpath = "github.com/dgraph-io/badger/v4",
        sum = "h1:kJrlajbXXL9DFTNuhhu9yCx7JJa4qpYWxtE8BzuWsEs=",
        version = "v4.2.0",
    )
    go_repository(
        name = "com_github_dgraph_io_ristretto",
        importpath = "github.com/dgraph-io/ristretto",
        sum = "h1:6CWw5tJNgpegArSHpNHJKldNeq03FQCwYvfMVWajOK8=",
        version = "v0.1.1",
    )
    go_repository(
        name = "com_github_dgryski_go_rendezvous",
        importpath = "github.com/dgryski/go-rendezvous",
        sum = "h1:lO4WD4F/rVNCu3HqELle0jiPLLBs70cWOduZpkS1E78=",
        version = "v0.0.0-20200823014737-9f7001d12a5f",
    )
    go_repository(
        name = "com_github_dustin_go_humanize",
        importpath = "github.com/dustin/go-humanize",
        sum = "h1:VSnTsYCnlFHaM2/igO1h6X3HA71jcobQuxemgkq4zYo=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_eclipse_paho_golang",
        importpath = "github.com/eclipse/paho.golang",
        sum = "h1:KHgl2wz6EJo7cMBmkuhpt7C576vP+kpPv7jjvSyR6Mk=",
        version = "v0.23.0",
    )
    go_repository(
        name = "com_github_eliben_go_sentencepiece",
        importpath = "github.com/eliben/go-sentencepiece",
        sum = "h1:wbnefMCxYyVYmeTVtiMJet+mS9CVwq5klveLpfQLsnk=",
        version = "v0.6.0",
    )
    go_repository(
        name = "com_github_envoyproxy_go_control_plane",
        importpath = "github.com/envoyproxy/go-control-plane",
        sum = "h1:K+fnvUM0VZ7ZFJf0n4L/BRlnsb9pL/GuDG6FqaH+PwM=",
        version = "v0.13.5-0.20251024222203-75eaa193e329",
    )
    go_repository(
        name = "com_github_envoyproxy_go_control_plane_envoy",
        importpath = "github.com/envoyproxy/go-control-plane/envoy",
        sum = "h1:ixjkELDE+ru6idPxcHLj8LBVc2bFP7iBytj353BoHUo=",
        version = "v1.35.0",
    )
    go_repository(
        name = "com_github_envoyproxy_go_control_plane_ratelimit",
        importpath = "github.com/envoyproxy/go-control-plane/ratelimit",
        sum = "h1:/G9QYbddjL25KvtKTv3an9lx6VBE2cnb8wp1vEGNYGI=",
        version = "v0.1.0",
    )
    go_repository(
        name = "com_github_envoyproxy_protoc_gen_validate",
        importpath = "github.com/envoyproxy/protoc-gen-validate",
        sum = "h1:DEo3O99U8j4hBFwbJfrz9VtgcDfUKS7KJ7spH3d86P8=",
        version = "v1.2.1",
    )
    go_repository(
        name = "com_github_felixge_httpsnoop",
        importpath = "github.com/felixge/httpsnoop",
        sum = "h1:NFTV2Zj1bL4mc9sqWACXbQFVBBg2W3GPvqp8/ESS2Wg=",
        version = "v1.0.4",
    )
    go_repository(
        name = "com_github_getsentry_sentry_go",
        importpath = "github.com/getsentry/sentry-go",
        sum = "h1:MtBW5H9QgdcJabtZcuJG80BMOwaBpkRDZkxRkNC1sN0=",
        version = "v0.18.0",
    )
    go_repository(
        name = "com_github_go_jose_go_jose_v4",
        importpath = "github.com/go-jose/go-jose/v4",
        sum = "h1:CVLmWDhDVRa6Mi/IgCgaopNosCaHz7zrMeF9MlZRkrs=",
        version = "v4.1.3",
    )
    go_repository(
        name = "com_github_go_logr_logr",
        importpath = "github.com/go-logr/logr",
        sum = "h1:CjnDlHq8ikf6E492q6eKboGOC0T8CDaOvkHCIg8idEI=",
        version = "v1.4.3",
    )
    go_repository(
        name = "com_github_go_logr_stdr",
        importpath = "github.com/go-logr/stdr",
        sum = "h1:hSWxHoqTgW2S2qGc0LTAI563KZ5YKYRhT3MFKZMbjag=",
        version = "v1.2.2",
    )
    go_repository(
        name = "com_github_go_redis_redis_v8",
        importpath = "github.com/go-redis/redis/v8",
        sum = "h1:AcZZR7igkdvfVmQTPnu9WE37LRrO/YrBH5zWyjDC0oI=",
        version = "v8.11.5",
    )
    go_repository(
        name = "com_github_goccy_go_yaml",
        importpath = "github.com/goccy/go-yaml",
        sum = "h1:PmFC1S6h8ljIz6gMRBopkjP1TVT7xuwrButHID66PoM=",
        version = "v1.19.2",
    )
    go_repository(
        name = "com_github_gogo_protobuf",
        importpath = "github.com/gogo/protobuf",
        sum = "h1:Ov1cvc58UF3b5XjBnZv7+opcTcQFZebYjWzi34vdm4Q=",
        version = "v1.3.2",
    )
    go_repository(
        name = "com_github_golang_glog",
        importpath = "github.com/golang/glog",
        sum = "h1:DrW6hGnjIhtvhOIiAKT6Psh/Kd/ldepEa81DKeiRJ5I=",
        version = "v1.2.5",
    )
    go_repository(
        name = "com_github_golang_groupcache",
        importpath = "github.com/golang/groupcache",
        sum = "h1:oI5xCqsCo564l8iNU+DwB5epxmsaqB+rhGL0m5jtYqE=",
        version = "v0.0.0-20210331224755-41bb18bfe9da",
    )
    go_repository(
        name = "com_github_golang_jwt_jwt_v5",
        importpath = "github.com/golang-jwt/jwt/v5",
        sum = "h1:OuVbFODueb089Lh128TAcimifWaLhJwVflnrgM17wHk=",
        version = "v5.2.1",
    )
    go_repository(
        name = "com_github_golang_protobuf",
        importpath = "github.com/golang/protobuf",
        sum = "h1:i7eJL8qZTpSEXOPTxNKhASYpMn+8e5Q6AdndVa1dWek=",
        version = "v1.5.4",
    )
    go_repository(
        name = "com_github_golang_snappy",
        importpath = "github.com/golang/snappy",
        sum = "h1:yAGX7huGHXlcLOEtBnF4w7FQwA26wojNCwOYAEhLjQM=",
        version = "v0.0.4",
    )
    go_repository(
        name = "com_github_google_flatbuffers",
        importpath = "github.com/google/flatbuffers",
        sum = "h1:MVlul7pQNoDzWRLTw5imwYsl+usrS1TXG2H4jg6ImGw=",
        version = "v1.12.1",
    )
    go_repository(
        name = "com_github_google_go_cmp",
        importpath = "github.com/google/go-cmp",
        sum = "h1:wk8382ETsv4JYUZwIsn6YpYiWiBsYLSJiTsyBybVuN8=",
        version = "v0.7.0",
    )
    go_repository(
        name = "com_github_google_go_pkcs11",
        importpath = "github.com/google/go-pkcs11",
        sum = "h1:PVRnTgtArZ3QQqTGtbtjtnIkzl2iY2kt24yqbrf7td8=",
        version = "v0.3.0",
    )
    go_repository(
        name = "com_github_google_jsonschema_go",
        importpath = "github.com/google/jsonschema-go",
        sum = "h1:tmrUohrwoLZZS/P3x7ex0WAVknEkBZM46iALbcqoRA8=",
        version = "v0.4.2",
    )
    go_repository(
        name = "com_github_google_martian_v3",
        importpath = "github.com/google/martian/v3",
        sum = "h1:DIhPTQrbPkgs2yJYdXU/eNACCG5DVQjySNRNlflZ9Fc=",
        version = "v3.3.3",
    )
    go_repository(
        name = "com_github_google_s2a_go",
        importpath = "github.com/google/s2a-go",
        sum = "h1:LGD7gtMgezd8a/Xak7mEWL0PjoTQFvpRudN895yqKW0=",
        version = "v0.1.9",
    )
    go_repository(
        name = "com_github_google_uuid",
        importpath = "github.com/google/uuid",
        sum = "h1:NIvaJDMOsjHA8n1jAhLSgzrAzy1Hgr+hNrb57e+94F0=",
        version = "v1.6.0",
    )
    go_repository(
        name = "com_github_googleapis_enterprise_certificate_proxy",
        importpath = "github.com/googleapis/enterprise-certificate-proxy",
        sum = "h1:zrn2Ee/nWmHulBx5sAVrGgAa0f2/R35S4DJwfFaUPFQ=",
        version = "v0.3.7",
    )
    go_repository(
        name = "com_github_googleapis_gax_go_v2",
        importpath = "github.com/googleapis/gax-go/v2",
        sum = "h1:iHbQmKLLZrexmb0OSsNGTeSTS0HO4YvFOG8g5E4Zd0Y=",
        version = "v2.16.0",
    )
    go_repository(
        name = "com_github_googlecloudplatform_opentelemetry_operations_go_detectors_gcp",
        importpath = "github.com/GoogleCloudPlatform/opentelemetry-operations-go/detectors/gcp",
        sum = "h1:sBEjpZlNHzK1voKq9695PJSX2o5NEXl7/OL3coiIY0c=",
        version = "v1.30.0",
    )
    go_repository(
        name = "com_github_gorilla_websocket",
        importpath = "github.com/gorilla/websocket",
        sum = "h1:saDtZ6Pbx/0u+bgYQ3q96pZgCzfhKXGPqt7kZ72aNNg=",
        version = "v1.5.3",
    )
    go_repository(
        name = "com_github_inconshreveable_mousetrap",
        importpath = "github.com/inconshreveable/mousetrap",
        sum = "h1:wN+x4NVGpMsO7ErUn/mUI3vEoE6Jt13X2s0bqwp9tc8=",
        version = "v1.1.0",
    )
    go_repository(
        name = "com_github_jinzhu_copier",
        importpath = "github.com/jinzhu/copier",
        sum = "h1:GlvfUwHk62RokgqVNvYsku0TATCF7bAHVwEXoBh3iJg=",
        version = "v0.3.5",
    )
    go_repository(
        name = "com_github_kaptinlin_jsonrepair",
        importpath = "github.com/kaptinlin/jsonrepair",
        sum = "h1:aPWX5HjnlEm7ZAlMRrlEWnWPc5ax2+4RlytDoGlGAm0=",
        version = "v0.2.6",
    )
    go_repository(
        name = "com_github_klauspost_compress",
        importpath = "github.com/klauspost/compress",
        sum = "h1:EF27CXIuDsYJ6mmvtBRlEuB2UVOqHG1tAXgZ7yIO+lw=",
        version = "v1.15.15",
    )
    go_repository(
        name = "com_github_kr_pretty",
        importpath = "github.com/kr/pretty",
        sum = "h1:flRD4NNwYAUpkphVc1HcthR4KEIFJ65n8Mw5qdRn3LE=",
        version = "v0.3.1",
    )
    go_repository(
        name = "com_github_kr_text",
        importpath = "github.com/kr/text",
        sum = "h1:5Nx0Ya0ZqY2ygV366QzturHI13Jq95ApcVaJBhpS+AY=",
        version = "v0.2.0",
    )
    go_repository(
        name = "com_github_kylelemons_godebug",
        importpath = "github.com/kylelemons/godebug",
        sum = "h1:RPNrshWIDI6G2gRW9EHilWtl7Z6Sb1BR0xunSBf0SNc=",
        version = "v1.1.0",
    )
    go_repository(
        name = "com_github_matttproud_golang_protobuf_extensions",
        importpath = "github.com/matttproud/golang_protobuf_extensions",
        sum = "h1:I0XW9+e1XWDxdcEniV4rQAIOPUGDq67JSCiRCgGCZLI=",
        version = "v1.0.2-0.20181231171920-c182affec369",
    )
    go_repository(
        name = "com_github_mochi_mqtt_server_v2",
        importpath = "github.com/mochi-mqtt/server/v2",
        sum = "h1:y0g4vrSLAag7T07l2oCzOa/+nKVLoazKEWAArwqBNYI=",
        version = "v2.7.9",
    )
    go_repository(
        name = "com_github_openai_openai_go",
        importpath = "github.com/openai/openai-go",
        sum = "h1:NBQCnXzqOTv5wsgNC36PrFEiskGfO5wccfCWDo9S1U0=",
        version = "v1.12.0",
    )
    go_repository(
        name = "com_github_pkg_browser",
        importpath = "github.com/pkg/browser",
        sum = "h1:+mdjkGKdHQG3305AYmdv1U2eRNDiU2ErMBj1gwrq8eQ=",
        version = "v0.0.0-20240102092130-5ac0b6a4141c",
    )
    go_repository(
        name = "com_github_pkg_errors",
        importpath = "github.com/pkg/errors",
        sum = "h1:FEBLx1zS214owpjy7qsBeixbURkuhQAwrK5UwLGTwt4=",
        version = "v0.9.1",
    )
    go_repository(
        name = "com_github_planetscale_vtprotobuf",
        importpath = "github.com/planetscale/vtprotobuf",
        sum = "h1:GFCKgmp0tecUJ0sJuv4pzYCqS9+RGSn52M3FUwPs+uo=",
        version = "v0.6.1-0.20240319094008-0393e58bdf10",
    )
    go_repository(
        name = "com_github_pmezard_go_difflib",
        importpath = "github.com/pmezard/go-difflib",
        sum = "h1:4DBwDE0NGyQoBHbLQYPwSUPoCMWR5BEzIk/f1lZbAQM=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_prometheus_client_golang",
        importpath = "github.com/prometheus/client_golang",
        sum = "h1:C+UIj/QWtmqY13Arb8kwMt5j34/0Z2iKamrJ+ryC0Gg=",
        version = "v1.12.0",
    )
    go_repository(
        name = "com_github_prometheus_client_model",
        importpath = "github.com/prometheus/client_model",
        sum = "h1:CmF68hwI0XsOQ5UwlBopMi2Ow4Pbg32akc4KIVCOm+Y=",
        version = "v0.2.1-0.20210607210712-147c58e9608a",
    )
    go_repository(
        name = "com_github_prometheus_common",
        importpath = "github.com/prometheus/common",
        sum = "h1:hWIdL3N2HoUx3B8j3YN9mWor0qhY/NlEKZEaXxuIRh4=",
        version = "v0.32.1",
    )
    go_repository(
        name = "com_github_prometheus_procfs",
        importpath = "github.com/prometheus/procfs",
        sum = "h1:4jVXhlkAyzOScmCkXBTOLRLTz8EeU+eyjrwB/EPq0VU=",
        version = "v0.7.3",
    )
    go_repository(
        name = "com_github_rogpeppe_go_internal",
        importpath = "github.com/rogpeppe/go-internal",
        sum = "h1:UQB4HGPB6osV0SQTLymcB4TgvyWu6ZyliaW0tI/otEQ=",
        version = "v1.14.1",
    )
    go_repository(
        name = "com_github_rs_xid",
        importpath = "github.com/rs/xid",
        sum = "h1:qd7wPTDkN6KQx2VmMBLrpHkiyQwgFXRnkOLacUiaSNY=",
        version = "v1.4.0",
    )
    go_repository(
        name = "com_github_russross_blackfriday_v2",
        importpath = "github.com/russross/blackfriday/v2",
        sum = "h1:JIOH55/0cWyOuilr9/qlrm0BSXldqnqwMsf35Ld67mk=",
        version = "v2.1.0",
    )
    go_repository(
        name = "com_github_spf13_cobra",
        importpath = "github.com/spf13/cobra",
        sum = "h1:DMTTonx5m65Ic0GOoRY2c16WCbHxOOw6xxezuLaBpcU=",
        version = "v1.10.2",
    )
    go_repository(
        name = "com_github_spf13_pflag",
        importpath = "github.com/spf13/pflag",
        sum = "h1:9exaQaMOCwffKiiiYk6/BndUBv+iRViNW+4lEMi0PvY=",
        version = "v1.0.9",
    )
    go_repository(
        name = "com_github_spiffe_go_spiffe_v2",
        importpath = "github.com/spiffe/go-spiffe/v2",
        sum = "h1:l+DolpxNWYgruGQVV0xsfeya3CsC7m8iBzDnMpsbLuo=",
        version = "v2.6.0",
    )
    go_repository(
        name = "com_github_stretchr_testify",
        importpath = "github.com/stretchr/testify",
        sum = "h1:7s2iGBzp5EwR7/aIZr8ao5+dra3wiQyKjjFuvgVKu7U=",
        version = "v1.11.1",
    )
    go_repository(
        name = "com_github_tidwall_gjson",
        importpath = "github.com/tidwall/gjson",
        sum = "h1:uo0p8EbA09J7RQaflQ1aBRffTR7xedD2bcIVSYxLnkM=",
        version = "v1.14.4",
    )
    go_repository(
        name = "com_github_tidwall_match",
        importpath = "github.com/tidwall/match",
        sum = "h1:+Ho715JplO36QYgwN9PGYNhgZvoUSc9X2c80KVTi+GA=",
        version = "v1.1.1",
    )
    go_repository(
        name = "com_github_tidwall_pretty",
        importpath = "github.com/tidwall/pretty",
        sum = "h1:qjsOFOWWQl+N3RsoF5/ssm1pHmJJwhjlSbZ51I6wMl4=",
        version = "v1.2.1",
    )
    go_repository(
        name = "com_github_tidwall_sjson",
        importpath = "github.com/tidwall/sjson",
        sum = "h1:kLy8mja+1c9jlljvWTlSazM7cKDRfJuR/bOJhcY5NcY=",
        version = "v1.2.5",
    )
    go_repository(
        name = "com_github_yuin_gopher_lua",
        importpath = "github.com/yuin/gopher-lua",
        sum = "h1:k/gmLsJDWwWqbLCur2yWnJzwQEKRcAHXo6seXGuSwWw=",
        version = "v0.0.0-20210529063254-f4c35e4016d9",
    )
    go_repository(
        name = "com_google_cloud_go",
        importpath = "cloud.google.com/go",
        sum = "h1:B3fRrSDkLRt5qSHWe40ERJvhvnQwdZiHu0bJOpldweE=",
        version = "v0.116.0",
    )
    go_repository(
        name = "com_google_cloud_go_auth",
        importpath = "cloud.google.com/go/auth",
        sum = "h1:74yCm7hCj2rUyyAocqnFzsAYXgJhrG26XCFimrc/Kz4=",
        version = "v0.17.0",
    )
    go_repository(
        name = "com_google_cloud_go_auth_oauth2adapt",
        importpath = "cloud.google.com/go/auth/oauth2adapt",
        sum = "h1:keo8NaayQZ6wimpNSmW5OPc283g65QNIiLpZnkHRbnc=",
        version = "v0.2.8",
    )
    go_repository(
        name = "com_google_cloud_go_compute_metadata",
        importpath = "cloud.google.com/go/compute/metadata",
        sum = "h1:pDUj4QMoPejqq20dK0Pg2N4yG9zIkYGdBtwLoEkH9Zs=",
        version = "v0.9.0",
    )
    go_repository(
        name = "com_google_cloud_go_iam",
        importpath = "cloud.google.com/go/iam",
        sum = "h1:kZKMKVNk/IsSSc/udOb83K0hL/Yh/Gcqpz+oAkoIFN8=",
        version = "v1.2.0",
    )
    go_repository(
        name = "com_google_cloud_go_longrunning",
        importpath = "cloud.google.com/go/longrunning",
        sum = "h1:xAe8+0YaWoCKr9t1+aWe+OeQgN/iJK1fEgZSXmjuEaE=",
        version = "v0.5.6",
    )
    go_repository(
        name = "com_google_cloud_go_storage",
        importpath = "cloud.google.com/go/storage",
        sum = "h1:CcxnSohZwizt4LCzQHWvBf1/kvtHUn7gk9QERXPyXFs=",
        version = "v1.43.0",
    )
    go_repository(
        name = "com_google_cloud_go_translate",
        importpath = "cloud.google.com/go/translate",
        sum = "h1:g+B29z4gtRGsiKDoTF+bNeH25bLRokAaElygX2FcZkE=",
        version = "v1.10.3",
    )
    go_repository(
        name = "dev_cel_expr",
        importpath = "cel.dev/expr",
        sum = "h1:56OvJKSH3hDGL0ml5uSxZmz3/3Pq4tJ+fb1unVLAFcY=",
        version = "v0.24.0",
    )
    go_repository(
        name = "in_gopkg_check_v1",
        importpath = "gopkg.in/check.v1",
        sum = "h1:Hei/4ADfdWqJk1ZMxUNpqntNwaWcugrBjAiHlqqRiVk=",
        version = "v1.0.0-20201130134442-10cb98267c6c",
    )
    go_repository(
        name = "in_gopkg_yaml_v3",
        importpath = "gopkg.in/yaml.v3",
        sum = "h1:fxVm/GzAzEWqLHuvctI91KS9hhNmmWOoWu0XTYJS7CA=",
        version = "v3.0.1",
    )
    go_repository(
        name = "in_yaml_go_yaml_v3",
        importpath = "go.yaml.in/yaml/v3",
        sum = "h1:tfq32ie2Jv2UxXFdLJdh3jXuOzWiL1fo0bu/FbuKpbc=",
        version = "v3.0.4",
    )
    go_repository(
        name = "io_etcd_go_bbolt",
        importpath = "go.etcd.io/bbolt",
        sum = "h1:XAzx9gjCb0Rxj7EoqcClPD1d5ZBxZJk0jbuoPHenBt0=",
        version = "v1.3.5",
    )
    go_repository(
        name = "io_opencensus_go",
        importpath = "go.opencensus.io",
        sum = "h1:y73uSU6J157QMP2kn2r30vwW1A2W2WFwSCGnAVxeaD0=",
        version = "v0.24.0",
    )
    go_repository(
        name = "io_opentelemetry_go_auto_sdk",
        importpath = "go.opentelemetry.io/auto/sdk",
        sum = "h1:jXsnJ4Lmnqd11kwkBV2LgLoFMZKizbCi5fNZ/ipaZ64=",
        version = "v1.2.1",
    )
    go_repository(
        name = "io_opentelemetry_go_contrib_detectors_gcp",
        importpath = "go.opentelemetry.io/contrib/detectors/gcp",
        sum = "h1:ZoYbqX7OaA/TAikspPl3ozPI6iY6LiIY9I8cUfm+pJs=",
        version = "v1.38.0",
    )
    go_repository(
        name = "io_opentelemetry_go_contrib_instrumentation_google_golang_org_grpc_otelgrpc",
        importpath = "go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc",
        sum = "h1:q4XOmH/0opmeuJtPsbFNivyl7bCt7yRBbeEm2sC/XtQ=",
        version = "v0.61.0",
    )
    go_repository(
        name = "io_opentelemetry_go_contrib_instrumentation_net_http_otelhttp",
        importpath = "go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp",
        sum = "h1:F7Jx+6hwnZ41NSFTO5q4LYDtJRXBf2PD0rNBkeB/lus=",
        version = "v0.61.0",
    )
    go_repository(
        name = "io_opentelemetry_go_otel",
        importpath = "go.opentelemetry.io/otel",
        sum = "h1:RkfdswUDRimDg0m2Az18RKOsnI8UDzppJAtj01/Ymk8=",
        version = "v1.38.0",
    )
    go_repository(
        name = "io_opentelemetry_go_otel_metric",
        importpath = "go.opentelemetry.io/otel/metric",
        sum = "h1:Kl6lzIYGAh5M159u9NgiRkmoMKjvbsKtYRwgfrA6WpA=",
        version = "v1.38.0",
    )
    go_repository(
        name = "io_opentelemetry_go_otel_sdk",
        importpath = "go.opentelemetry.io/otel/sdk",
        sum = "h1:l48sr5YbNf2hpCUj/FoGhW9yDkl+Ma+LrVl8qaM5b+E=",
        version = "v1.38.0",
    )
    go_repository(
        name = "io_opentelemetry_go_otel_sdk_metric",
        importpath = "go.opentelemetry.io/otel/sdk/metric",
        sum = "h1:aSH66iL0aZqo//xXzQLYozmWrXxyFkBJ6qT5wthqPoM=",
        version = "v1.38.0",
    )
    go_repository(
        name = "io_opentelemetry_go_otel_trace",
        importpath = "go.opentelemetry.io/otel/trace",
        sum = "h1:Fxk5bKrDZJUH+AMyyIXGcFAPah0oRcT+LuNtJrmcNLE=",
        version = "v1.38.0",
    )
    go_repository(
        name = "org_golang_google_api",
        importpath = "google.golang.org/api",
        sum = "h1:8Y0lzvHlZps53PEaw+G29SsQIkuKrumGWs9puiexNAA=",
        version = "v0.257.0",
    )
    go_repository(
        name = "org_golang_google_appengine",
        importpath = "google.golang.org/appengine",
        sum = "h1:IhEN5q69dyKagZPYMSdIjS2HqprW324FRQZJcGqPAsM=",
        version = "v1.6.8",
    )
    go_repository(
        name = "org_golang_google_genai",
        importpath = "google.golang.org/genai",
        sum = "h1:XFHfo0DDCzdzQALZoFs6nowAHO2cE95XyVvFLNaFLRY=",
        version = "v1.42.0",
    )
    go_repository(
        name = "org_golang_google_genproto",
        importpath = "google.golang.org/genproto",
        sum = "h1:GvESR9BIyHUahIb0NcTum6itIWtdoglGX+rnGxm2934=",
        version = "v0.0.0-20251202230838-ff82c1b0f217",
    )
    go_repository(
        name = "org_golang_google_genproto_googleapis_api",
        importpath = "google.golang.org/genproto/googleapis/api",
        sum = "h1:fCvbg86sFXwdrl5LgVcTEvNC+2txB5mgROGmRL5mrls=",
        version = "v0.0.0-20251202230838-ff82c1b0f217",
    )
    go_repository(
        name = "org_golang_google_genproto_googleapis_bytestream",
        importpath = "google.golang.org/genproto/googleapis/bytestream",
        sum = "h1:7FlucM2tFADtEDnIlDrR12KdRqV48B1GSTU1U6uKSiY=",
        version = "v0.0.0-20251124214823-79d6a2a48846",
    )
    go_repository(
        name = "org_golang_google_genproto_googleapis_rpc",
        importpath = "google.golang.org/genproto/googleapis/rpc",
        sum = "h1:gRkg/vSppuSQoDjxyiGfN4Upv/h/DQmIR10ZU8dh4Ww=",
        version = "v0.0.0-20251202230838-ff82c1b0f217",
    )
    go_repository(
        name = "org_golang_google_grpc",
        importpath = "google.golang.org/grpc",
        sum = "h1:wVVY6/8cGA6vvffn+wWK5ToddbgdU3d8MNENr4evgXM=",
        version = "v1.77.0",
    )
    go_repository(
        name = "org_golang_google_protobuf",
        importpath = "google.golang.org/protobuf",
        sum = "h1:fV6ZwhNocDyBLK0dj+fg8ektcVegBBuEolpbTQyBNVE=",
        version = "v1.36.11",
    )
    go_repository(
        name = "org_golang_x_crypto",
        importpath = "golang.org/x/crypto",
        sum = "h1:jMBrvKuj23MTlT0bQEOBcAE0mjg8mK9RXFhRH6nyF3Q=",
        version = "v0.45.0",
    )
    go_repository(
        name = "org_golang_x_exp",
        importpath = "golang.org/x/exp",
        sum = "h1:UA2aFVmmsIlefxMk29Dp2juaUSth8Pyn3Tq5Y5mJGME=",
        version = "v0.0.0-20230626212559-97b1e661b5df",
    )
    go_repository(
        name = "org_golang_x_mod",
        importpath = "golang.org/x/mod",
        sum = "h1:HV8lRxZC4l2cr3Zq1LvtOsi/ThTgWnUk/y64QSs8GwA=",
        version = "v0.29.0",
    )
    go_repository(
        name = "org_golang_x_net",
        importpath = "golang.org/x/net",
        sum = "h1:Mx+4dIFzqraBXUugkia1OOvlD6LemFo1ALMHjrXDOhY=",
        version = "v0.47.0",
    )
    go_repository(
        name = "org_golang_x_oauth2",
        importpath = "golang.org/x/oauth2",
        sum = "h1:4Q+qn+E5z8gPRJfmRy7C2gGG3T4jIprK6aSYgTXGRpo=",
        version = "v0.33.0",
    )
    go_repository(
        name = "org_golang_x_sync",
        importpath = "golang.org/x/sync",
        sum = "h1:kr88TuHDroi+UVf+0hZnirlk8o8T+4MrK6mr60WkH/I=",
        version = "v0.18.0",
    )
    go_repository(
        name = "org_golang_x_sys",
        importpath = "golang.org/x/sys",
        sum = "h1:3yZWxaJjBmCWXqhN1qh02AkOnCQ1poK6oF+a7xWL6Gc=",
        version = "v0.38.0",
    )
    go_repository(
        name = "org_golang_x_term",
        importpath = "golang.org/x/term",
        sum = "h1:8EGAD0qCmHYZg6J17DvsMy9/wJ7/D/4pV/wfnld5lTU=",
        version = "v0.37.0",
    )
    go_repository(
        name = "org_golang_x_text",
        importpath = "golang.org/x/text",
        sum = "h1:aC8ghyu4JhP8VojJ2lEHBnochRno1sgL6nEi9WGFGMM=",
        version = "v0.31.0",
    )
    go_repository(
        name = "org_golang_x_time",
        importpath = "golang.org/x/time",
        sum = "h1:MRx4UaLrDotUKUdCIqzPC48t1Y9hANFKIRpNx+Te8PI=",
        version = "v0.14.0",
    )
    go_repository(
        name = "org_golang_x_tools",
        importpath = "golang.org/x/tools",
        sum = "h1:Hx2Xv8hISq8Lm16jvBZ2VQf+RLmbd7wVUsALibYI/IQ=",
        version = "v0.38.0",
    )
    go_repository(
        name = "org_gonum_v1_gonum",
        importpath = "gonum.org/v1/gonum",
        sum = "h1:5+ul4Swaf3ESvrOnidPp4GZbzf0mxVQpDCYUQE7OJfk=",
        version = "v0.16.0",
    )
    go_repository(
        name = "org_uber_go_goleak",
        importpath = "go.uber.org/goleak",
        sum = "h1:2K3zAYmnTNqV73imy9J1T3WC+gmCePx2hEGkimedGto=",
        version = "v1.3.0",
    )
