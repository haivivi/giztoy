# HarmonyOS Bazel 示例

本目录包含 HarmonyOS 应用开发的命令行构建示例。

## ✅ 纯命令行开发流程

**无需使用 DevEco Studio GUI！** 只需安装 DevEco Studio 获取 SDK 和工具链即可。

### 快速开始

```bash
# 1. 构建并运行
./build_native.sh run

# 2. 单独构建
./build_native.sh build

# 3. 清理构建
./build_native.sh clean
```

### 环境变量设置（手动运行时需要）

```bash
export PATH="/Applications/DevEco-Studio.app/Contents/tools/hvigor/bin:$PATH"
export PATH="/Applications/DevEco-Studio.app/Contents/tools/ohpm/bin:$PATH"
export DEVECO_SDK_HOME="/Applications/DevEco-Studio.app/Contents/sdk"
export JAVA_HOME="/Applications/DevEco-Studio.app/Contents/jbr/Contents/Home"
export PATH="$JAVA_HOME/bin:$PATH"
```

### 手动构建步骤

```bash
cd HelloWorld

# 安装依赖
ohpm install

# 构建 HAP
hvigorw assembleHap --no-daemon -p product=default

# 安装到模拟器
hdc install entry/build/default/outputs/default/entry-default-unsigned.hap

# 启动应用
hdc shell aa start -a EntryAbility -b com.example.hellobazel
```

## 📂 项目结构

```
harmonyos/
├── HelloWorld/                 # HarmonyOS 项目
│   ├── entry/                  # 入口模块
│   │   └── src/main/
│   │       ├── ets/            # ArkTS 源代码
│   │       │   ├── pages/      # 页面
│   │       │   └── entryability/ # Ability
│   │       ├── resources/      # 资源文件
│   │       └── module.json5    # 模块配置
│   ├── AppScope/               # 应用级配置
│   │   ├── app.json5           # 应用配置
│   │   └── resources/          # 应用资源
│   ├── build-profile.json5     # 构建配置
│   ├── hvigor/                 # hvigor 配置
│   │   └── hvigor-config.json5
│   └── oh-package.json5        # 依赖配置
├── build_native.sh             # 构建脚本
├── sign_hap.sh                 # 签名脚本（可选）
└── README.md
```

## 🔧 常用命令

### hdc 设备管理

```bash
# 列出设备
hdc list targets

# 安装应用
hdc install <hap文件>

# 卸载应用
hdc uninstall <包名>

# 启动应用
hdc shell aa start -a <Ability名> -b <包名>

# 查看已安装应用
hdc shell bm dump -a

# 查看日志
hdc hilog
```

### hvigorw 构建命令

```bash
# 构建 HAP
hvigorw assembleHap -p product=default

# 构建 APP
hvigorw assembleApp -p product=default

# 清理
hvigorw clean

# 查看帮助
hvigorw --help
```

## 📱 模拟器管理

模拟器需要在 DevEco Studio 中创建和启动：
- Tools → Device Manager → Create Device

启动后，可以用 `hdc` 命令行操作。

## ⚠️ 注意事项

1. **无需签名**: 模拟器允许安装未签名的 HAP
2. **真机部署**: 需要配置签名（修改 `build-profile.json5`）
3. **项目模板**: HelloWorld 项目基于 DevEco Studio 的 previewProjectTemplate

## 🔑 签名配置（真机部署）

编辑 `HelloWorld/build-profile.json5`:

```json5
{
  "app": {
    "signingConfigs": [
      {
        "name": "default",
        "type": "HarmonyOS",
        "material": {
          "storeFile": "/path/to/keystore.p12",
          "storePassword": "password",
          "keyAlias": "alias",
          "keyPassword": "password",
          "certpath": "/path/to/cert.cer",
          "profile": "/path/to/profile.p7b"
        }
      }
    ]
  }
}
```
