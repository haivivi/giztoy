"""HarmonyOS Bazel Providers

定义 HarmonyOS 构建过程中使用的 Provider。
"""

HarmonyOSHapInfo = provider(
    doc = "HarmonyOS HAP 包信息",
    fields = {
        "hap": "HAP 文件",
        "unsigned_hap": "未签名的 HAP 文件",
        "bundle_name": "Bundle 名称",
        "module_name": "模块名称",
        "is_signed": "是否已签名",
    },
)

HarmonyOSLibraryInfo = provider(
    doc = "HarmonyOS HAR 库信息",
    fields = {
        "har": "HAR 文件",
        "module_name": "模块名称",
        "deps": "依赖列表",
    },
)

HarmonyOSSigningInfo = provider(
    doc = "HarmonyOS 签名配置信息",
    fields = {
        "keystore": "密钥库文件 (.p12)",
        "keystore_password": "密钥库密码",
        "key_alias": "密钥别名",
        "key_password": "密钥密码",
        "cert_file": "证书文件",
        "profile": "签名 profile 文件 (.p7b)",
    },
)

HarmonyOSToolchainInfo = provider(
    doc = "HarmonyOS 工具链信息",
    fields = {
        "sdk_path": "SDK 路径",
        "hvigorw": "hvigorw 工具路径",
        "ohpm": "ohpm 工具路径",
        "hdc": "hdc 工具路径",
        "java_home": "Java Home 路径",
        "sign_tool": "签名工具路径",
        "pack_tool": "打包工具路径",
    },
)
