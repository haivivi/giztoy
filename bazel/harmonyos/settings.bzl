"""HarmonyOS Build Settings

通过 Bazel build settings 配置 HarmonyOS 工具链路径。

## 配置方式

### 方式 1: 通过 .bazelrc 配置（推荐）

在 .bazelrc 中添加：
```
build --@//bazel/harmonyos:deveco_home=/Applications/DevEco-Studio.app/Contents
build --@//bazel/harmonyos:sdk_home=/path/to/sdk
```

### 方式 2: 通过命令行配置
```bash
bazel build //... --@//bazel/harmonyos:deveco_home=/path/to/deveco
```

### 方式 3: 通过环境变量
```bash
export DEVECO_HOME=/Applications/DevEco-Studio.app/Contents
bazel build //...
```
"""

load("@bazel_skylib//rules:common_settings.bzl", "string_flag")

def harmonyos_settings():
    """定义 HarmonyOS 相关的 build settings"""
    
    # DevEco Studio 安装路径
    string_flag(
        name = "deveco_home",
        build_setting_default = "",
        visibility = ["//visibility:public"],
    )
    
    # SDK 路径（可选，默认使用 DevEco 内置）
    string_flag(
        name = "sdk_home",
        build_setting_default = "",
        visibility = ["//visibility:public"],
    )
    
    # Java Home 路径（可选，默认使用 DevEco 内置）
    string_flag(
        name = "java_home",
        build_setting_default = "",
        visibility = ["//visibility:public"],
    )

# Build setting labels
DEVECO_HOME_SETTING = "//bazel/harmonyos:deveco_home"
SDK_HOME_SETTING = "//bazel/harmonyos:sdk_home"
JAVA_HOME_SETTING = "//bazel/harmonyos:java_home"
