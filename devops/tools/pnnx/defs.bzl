"""PNNX (PyTorch Neural Network Exchange) tool repository rule.

Downloads the pre-built PNNX binary for converting ONNX models to ncnn format.
Used by //devops/tools/pnnx:convert to generate speaker embedding model files.

Source: https://github.com/pnnx/pnnx
"""

_PNNX_VERSION = "20260112"

_PNNX_SHA256 = {
    "darwin_amd64": "1106a41e35f98ff90892fbac924a6c8f4bc62c1f38498db785e72eb82d48d080",
    "darwin_arm64": "1106a41e35f98ff90892fbac924a6c8f4bc62c1f38498db785e72eb82d48d080",
    "linux_amd64": "52de0d077f1c30ae2f828f5596e16d632e0ff82a136601adf194a98042427f9b",
    "linux_arm64": "2a3b588240203e311bdfd54de950342726ab234914dd46f8aed4da606b0cf762",
}

_PNNX_ARCHIVE = {
    "darwin_amd64": "pnnx-{}-macos".format(_PNNX_VERSION),
    "darwin_arm64": "pnnx-{}-macos".format(_PNNX_VERSION),
    "linux_amd64": "pnnx-{}-linux".format(_PNNX_VERSION),
    "linux_arm64": "pnnx-{}-linux-aarch64".format(_PNNX_VERSION),
}

def _pnnx_repo_impl(ctx):
    """Download pre-built PNNX binary."""
    os = ctx.os.name.lower()
    arch = ctx.os.arch

    if os in ("mac os x", "darwin", "macos") or os.startswith("darwin"):
        os_name = "darwin"
    elif os in ("linux",) or os.startswith("linux"):
        os_name = "linux"
    else:
        fail("Unsupported OS: '{}'".format(os))

    if arch == "amd64" or arch == "x86_64":
        arch_name = "amd64"
    elif arch == "aarch64" or arch == "arm64":
        arch_name = "arm64"
    else:
        fail("Unsupported architecture: " + arch)

    platform = "{}_{}".format(os_name, arch_name)
    prefix = _PNNX_ARCHIVE[platform]

    ctx.download_and_extract(
        url = "https://github.com/pnnx/pnnx/releases/download/{}/{}.zip".format(
            _PNNX_VERSION,
            prefix,
        ),
        sha256 = _PNNX_SHA256[platform],
        type = "zip",
    )

    ctx.execute(["chmod", "+x", "{}/pnnx".format(prefix)])
    ctx.symlink("{}/pnnx".format(prefix), "pnnx")

    ctx.file("BUILD.bazel", """\
package(default_visibility = ["//visibility:public"])

exports_files(["pnnx"])
""")

pnnx_repo = repository_rule(
    implementation = _pnnx_repo_impl,
)
