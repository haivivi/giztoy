"""Mermaid.js repository rule for documentation diagrams."""

_MERMAID_VERSION = "11.4.1"

def _mermaid_repo_impl(ctx):
    """Download mermaid.min.js from CDN."""
    ctx.download(
        url = "https://cdn.jsdelivr.net/npm/mermaid@{}/dist/mermaid.min.js".format(_MERMAID_VERSION),
        sha256 = "a43bc1afd446f9c4cc66ac5dd45d02e8d65e26fc5344ec0ef787f88d6ddb6f9e",
        output = "mermaid.min.js",
    )

    ctx.file("BUILD.bazel", """
package(default_visibility = ["//visibility:public"])

exports_files(["mermaid.min.js"])
""")

mermaid_repo = repository_rule(
    implementation = _mermaid_repo_impl,
    attrs = {},
)
