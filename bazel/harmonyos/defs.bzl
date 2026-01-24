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
        **kwargs):
    """便捷宏：构建并可选安装 HarmonyOS 应用
    
    这个宏会创建以下 targets:
    - {name}_hap: 构建 HAP 包
    - {name}_signed: 签名的 HAP 包（如果 sign=True）
    - {name}: 安装脚本
    
    Args:
        name: Target 名称
        project_dir: HarmonyOS 项目目录
        bundle_name: Bundle 名称
        srcs: 源文件
        resources: 资源文件
        module_name: 模块名称
        product: 构建产品配置
        ability_name: 启动的 Ability 名称
        sign: 是否签名
        **kwargs: 其他参数
        
    Note:
        需要设置环境变量 DEVECO_HOME 指向 DevEco Studio 安装路径
    """
    
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
        )
        install_hap = ":" + signed_target
    
    # 安装脚本
    harmonyos_install(
        name = name,
        hap = install_hap,
        bundle_name = bundle_name,
        ability_name = ability_name,
    )
