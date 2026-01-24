# HarmonyOS Bazel Rules

用于构建 HarmonyOS 应用的 Bazel rules。

## 🔧 环境配置

**Bazel 自动下载工具链**，无需手动安装！

### 工作原理

```
MODULE.bazel 中注册 @hos_sdk extension
        ↓
Bazel 自动下载 hos-sdk 命令行工具
        ↓
构建时自动使用
```

### 本地开发

```bash
# 直接运行，Bazel 自动处理一切
bazel build //examples/bazel/harmonyos:hello_bazel_hap
bazel run //examples/bazel/harmonyos:hello_bazel
```

如果本地安装了 DevEco Studio，会优先使用它（更快）：
- 日志显示：`使用 DevEco Studio 工具链: /Applications/DevEco-Studio.app/Contents`

如果没有 DevEco Studio，使用 @hos_sdk：
- 日志显示：`使用 Bazel @hos_sdk: external/hos_sdk`

### CI 环境

CI 只需要安装 Java、Node.js 和 hvigor：

```yaml
# .github/workflows/build.yaml
- uses: actions/setup-java@v4
  with:
    java-version: '17'
- uses: actions/setup-node@v4
  with:
    node-version: '18'
- run: npm install -g @ohos/hvigor-ohos-plugin @ohos/hvigor
- run: bazel build //...  # Bazel 自动下载 @hos_sdk
```

### 工具来源对比

| 环境 | ohpm | hvigorw | hdc/证书 |
|------|------|---------|----------|
| DevEco Studio | `$DEVECO_HOME/tools/ohpm` | `$DEVECO_HOME/tools/hvigor` | 内置 SDK |
| Bazel @hos_sdk | 自动下载 | `npm install` | 自动下载+安装 |

## 📦 Rules 列表

| Rule | 功能 | 说明 |
|------|------|------|
| `harmonyos_hap` | 构建 HAP 包 | 调用 hvigorw 构建 |
| `harmonyos_sign` | 签名 HAP 包 | 使用 hap-sign-tool |
| `harmonyos_install` | 安装到设备 | 调用 hdc 安装 |
| `harmonyos_application` | 便捷宏 | 一站式构建+安装 |

## 🚀 快速开始

### 1. 配置环境

在 `.bazelrc` 中添加：

```bash
build --action_env=DEVECO_HOME=/Applications/DevEco-Studio.app/Contents
```

### 2. 使用 Rules

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

### 3. 构建和运行

```bash
# 构建 HAP
bazel build //:my_app_hap

# 安装并运行
bazel run //:my_app
```

## 📖 Rule 参考

### harmonyos_hap

构建 HarmonyOS HAP 包。

```python
harmonyos_hap(
    name = "my_hap",
    project_dir = "path/to/project",           # 必需：项目目录
    bundle_name = "com.example.app",           # 必需：Bundle 名称
    srcs = glob(["**/*.ets"]),                 # 源文件
    resources = glob(["**/resources/**"]),     # 资源文件
    module_name = "entry",                     # 模块名（默认 entry）
    product = "default",                       # 产品配置（默认 default）
)
```

### harmonyos_sign

签名 HAP 包。

```python
harmonyos_sign(
    name = "my_signed_hap",
    hap = ":my_hap",                           # 必需：输入 HAP
    bundle_name = "com.example.app",           # 必需：Bundle 名称
    
    # 可选：自定义签名（默认使用 OpenHarmony 调试签名）
    keystore = "path/to/keystore.p12",
    keystore_password = "password",
    key_alias = "my-key",
    key_password = "password",
    profile_template = "path/to/profile.json",
)
```

### harmonyos_install

安装并启动应用。

```python
harmonyos_install(
    name = "install",
    hap = ":my_hap",                           # 必需：HAP 文件
    bundle_name = "com.example.app",           # 必需：Bundle 名称
    ability_name = "EntryAbility",             # 默认 EntryAbility
)
```

### harmonyos_application (宏)

便捷宏，自动创建多个 targets。

```python
harmonyos_application(
    name = "my_app",
    project_dir = "path/to/project",
    bundle_name = "com.example.app",
    srcs = glob(["**/*.ets"]),
    resources = glob(["**/resources/**"]),
    sign = False,                              # 是否签名
    ability_name = "EntryAbility",
)
```

生成的 targets:
- `my_app_hap` - HAP 包
- `my_app_signed` - 签名的 HAP（如果 sign=True）
- `my_app` - 安装脚本（可执行）

## 🔑 环境变量说明

| 变量 | 必需 | 说明 | 默认值 |
|------|------|------|--------|
| `DEVECO_HOME` | ✅ | DevEco Studio 安装路径 | - |
| `HARMONYOS_SDK_HOME` | ❌ | SDK 路径 | `$DEVECO_HOME/sdk` |
| `JAVA_HOME` | ❌ | Java 路径 | `$DEVECO_HOME/jbr/Contents/Home` |

## 📝 Providers

```python
load("//bazel/harmonyos:defs.bzl", "HarmonyOSHapInfo")

# HarmonyOSHapInfo 字段:
# - hap: HAP 文件
# - unsigned_hap: 未签名的 HAP 文件
# - bundle_name: Bundle 名称
# - module_name: 模块名称
# - is_signed: 是否已签名
```

## ⚠️ 注意事项

1. **模拟器不需要签名** - 开发时可以跳过 `harmonyos_sign`
2. **真机需要签名** - 使用 `sign = True` 或单独调用 `harmonyos_sign`
3. **项目结构要求** - 必须是有效的 HarmonyOS 项目（包含 `build-profile.json5` 等）
4. **设备连接** - 运行前确保设备已连接（`hdc list targets`）
