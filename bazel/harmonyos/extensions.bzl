"""HarmonyOS Bazel Extensions

Bzlmod 扩展，用于自动下载 HarmonyOS SDK。
"""

load(":repository.bzl", "hos_sdk", "hos_sdk_repository")

# 导出 module extension
hos_sdk_extension = hos_sdk
