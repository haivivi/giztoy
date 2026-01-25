"""HarmonyOS Bazel Rules

用于构建 HarmonyOS 应用的 Bazel rules。

## 环境配置

所有路径通过环境变量配置，不硬编码。

### 必需环境变量

```bash
export DEVECO_HOME=/Applications/DevEco-Studio.app/Contents
```

### 可选环境变量

```bash
export HARMONYOS_SDK_HOME=/path/to/sdk      # 默认: $DEVECO_HOME/sdk
export JAVA_HOME=/path/to/java              # 默认: $DEVECO_HOME/jbr/Contents/Home
```

### 通过 .bazelrc 配置（推荐）

在 .bazelrc 中添加：
```
# HarmonyOS 工具链配置
build --action_env=DEVECO_HOME=/Applications/DevEco-Studio.app/Contents
```

## 快速开始

```python
load("//bazel/harmonyos:defs.bzl", "harmonyos_application")

harmonyos_application(
    name = "my_app",
    project_dir = "path/to/harmonyos/project",
    bundle_name = "com.example.myapp",
    srcs = glob(["**/*.ets"]),
    resources = glob(["**/resources/**"]),
)
```

运行:
```bash
bazel build //:my_app_hap
bazel run //:my_app
```

## Rules

- `harmonyos_hap`: 构建 HAP 包
- `harmonyos_sign`: 签名 HAP 包
- `harmonyos_install`: 安装到设备
- `harmonyos_application`: 便捷宏
"""

load(
    ":rules.bzl",
    _harmonyos_hap = "harmonyos_hap",
    _harmonyos_install = "harmonyos_install",
    _harmonyos_sign = "harmonyos_sign",
)
load(
    ":providers.bzl",
    _HarmonyOSHapInfo = "HarmonyOSHapInfo",
    _HarmonyOSLibraryInfo = "HarmonyOSLibraryInfo",
    _HarmonyOSSigningInfo = "HarmonyOSSigningInfo",
    _HarmonyOSToolchainInfo = "HarmonyOSToolchainInfo",
)
load(
    ":toolchain.bzl",
    _get_toolchain_env_script = "get_toolchain_env_script",
    _get_toolchain_paths = "get_toolchain_paths",
    _harmonyos_toolchain = "harmonyos_toolchain",
)

# Rules
harmonyos_hap = _harmonyos_hap
harmonyos_sign = _harmonyos_sign
harmonyos_install = _harmonyos_install
harmonyos_toolchain = _harmonyos_toolchain

# Providers
HarmonyOSHapInfo = _HarmonyOSHapInfo
HarmonyOSLibraryInfo = _HarmonyOSLibraryInfo
HarmonyOSSigningInfo = _HarmonyOSSigningInfo
HarmonyOSToolchainInfo = _HarmonyOSToolchainInfo

# Utilities
get_toolchain_paths = _get_toolchain_paths
get_toolchain_env_script = _get_toolchain_env_script

def harmonyos_application(
        name,
        project_dir,
        bundle_name,
        srcs = [],
        resources = [],
        module_name = "entry",
        product = "default",
        ability_name = "EntryAbility",
        sign = False,
        tags = [],
        **kwargs):
    """Convenience macro: build and optionally install HarmonyOS application.

    This macro creates the following targets:
    - {name}_hap: Build HAP package
    - {name}_signed: Signed HAP package (if sign=True)
    - {name}: Install script
    
    Args:
        name: Target name
        project_dir: HarmonyOS project directory
        bundle_name: Bundle name
        srcs: Source files
        resources: Resource files
        module_name: Module name
        product: Build product configuration
        ability_name: Entry Ability name
        sign: Whether to sign the HAP
        tags: Additional tags (already includes "manual" by default)
        **kwargs: Additional arguments
        
    Note:
        Requires DEVECO_HOME environment variable pointing to DevEco Studio.
        Adds "manual" tag by default, excluded from `bazel build //...`.
        Use explicit build: `bazel build //path:target`
    """
    
    # Add "manual" tag by default (requires DevEco Studio, not available in CI)
    all_tags = ["manual"] + tags
    
    hap_target = name + "_hap"
    
    # 构建 HAP
    harmonyos_hap(
        name = hap_target,
        project_dir = project_dir,
        bundle_name = bundle_name,
        srcs = srcs,
        resources = resources,
        module_name = module_name,
        product = product,
        tags = all_tags,
        **kwargs
    )
    
    install_hap = ":" + hap_target
    
    # 可选签名
    if sign:
        signed_target = name + "_signed"
        harmonyos_sign(
            name = signed_target,
            hap = ":" + hap_target,
            bundle_name = bundle_name,
            module_name = module_name,
            tags = all_tags,
        )
        install_hap = ":" + signed_target
    
    # 安装脚本
    harmonyos_install(
        name = name,
        hap = install_hap,
        bundle_name = bundle_name,
        ability_name = ability_name,
        tags = all_tags,
    )
