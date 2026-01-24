"""HarmonyOS Toolchain 配置

配置 HarmonyOS 开发所需的工具链。
路径通过 build settings 或环境变量配置，不硬编码。
"""

load(":providers.bzl", "HarmonyOSToolchainInfo")

# ============================================================================
# Build Settings 引用
# ============================================================================

DEVECO_HOME_SETTING = "//bazel/harmonyos:deveco_home"
SDK_HOME_SETTING = "//bazel/harmonyos:sdk_home"
JAVA_HOME_SETTING = "//bazel/harmonyos:java_home"

# ============================================================================
# Toolchain Rule
# ============================================================================

def _harmonyos_toolchain_impl(ctx):
    """HarmonyOS 工具链实现
    
    路径优先级：
    1. Rule 属性直接指定
    2. Build settings (通过 .bazelrc)
    3. 环境变量
    """
    
    # 获取路径配置（不使用硬编码默认值）
    deveco_home = ctx.attr.deveco_home
    sdk_path = ctx.attr.sdk_home
    java_home = ctx.attr.java_home
    
    # 验证必需的路径
    if not deveco_home:
        fail("""
DevEco Home 路径未配置。请通过以下方式之一配置：

1. 在 .bazelrc 中添加：
   build --@//bazel/harmonyos:deveco_home=/Applications/DevEco-Studio.app/Contents

2. 或通过命令行：
   bazel build //... --@//bazel/harmonyos:deveco_home=/path/to/deveco

3. 或设置环境变量 DEVECO_HOME
""")
    
    # SDK 路径默认使用 DevEco 内置
    if not sdk_path:
        sdk_path = deveco_home + "/sdk"
    
    # Java 路径默认使用 DevEco 内置
    if not java_home:
        java_home = deveco_home + "/jbr/Contents/Home"
    
    return [
        platform_common.ToolchainInfo(
            harmonyos = HarmonyOSToolchainInfo(
                sdk_path = sdk_path,
                hvigorw = deveco_home + "/tools/hvigor/bin/hvigorw",
                ohpm = deveco_home + "/tools/ohpm/bin/ohpm",
                hdc = sdk_path + "/default/openharmony/toolchains/hdc",
                java_home = java_home,
                sign_tool = sdk_path + "/default/openharmony/toolchains/lib/hap-sign-tool.jar",
                pack_tool = sdk_path + "/default/openharmony/toolchains/lib/app_packing_tool.jar",
            ),
        ),
    ]

harmonyos_toolchain = rule(
    implementation = _harmonyos_toolchain_impl,
    attrs = {
        "deveco_home": attr.string(
            doc = "DevEco Studio 安装路径",
        ),
        "sdk_home": attr.string(
            doc = "HarmonyOS SDK 路径（可选，默认使用 DevEco Studio 内置 SDK）",
        ),
        "java_home": attr.string(
            doc = "Java Home 路径（可选，默认使用 DevEco Studio 内置）",
        ),
    },
    doc = "定义 HarmonyOS 工具链",
)

# ============================================================================
# 工具链路径获取函数
# ============================================================================

def get_toolchain_paths(deveco_home = None, sdk_home = None, java_home = None):
    """获取工具链路径配置
    
    Args:
        deveco_home: DevEco Studio 安装路径
        sdk_home: SDK 路径（可选）
        java_home: Java 路径（可选）
        
    Returns:
        包含所有工具路径的字典
        
    Note:
        如果未指定路径，会尝试从环境变量获取：
        - DEVECO_HOME
        - HARMONYOS_SDK_HOME
        - JAVA_HOME
    """
    
    # 路径不在这里硬编码，由调用者或环境变量提供
    if not deveco_home:
        # 返回占位符，实际值在运行时通过环境变量获取
        deveco_home = "${DEVECO_HOME}"
    
    if not sdk_home:
        sdk_home = deveco_home + "/sdk"
    
    if not java_home:
        java_home = deveco_home + "/jbr/Contents/Home"
    
    return {
        "deveco_home": deveco_home,
        "sdk_path": sdk_home,
        "hvigorw": deveco_home + "/tools/hvigor/bin/hvigorw",
        "ohpm": deveco_home + "/tools/ohpm/bin/ohpm",
        "hdc": sdk_home + "/default/openharmony/toolchains/hdc",
        "java_home": java_home,
        "java": java_home + "/bin/java",
        "keytool": java_home + "/bin/keytool",
        "sign_tool": sdk_home + "/default/openharmony/toolchains/lib/hap-sign-tool.jar",
        "pack_tool": sdk_home + "/default/openharmony/toolchains/lib/app_packing_tool.jar",
        "keystore": sdk_home + "/default/openharmony/toolchains/lib/OpenHarmony.p12",
        "profile_cert": sdk_home + "/default/openharmony/toolchains/lib/OpenHarmonyProfileDebug.pem",
        "profile_template": sdk_home + "/default/openharmony/toolchains/lib/UnsgnedDebugProfileTemplate.json",
    }

def get_toolchain_env_script():
    """生成设置环境变量的脚本片段
    
    支持三种环境（按优先级）:
    1. Bazel external: @hos_sdk (自动下载)
    2. 本地开发: DevEco Studio (DEVECO_HOME)
    3. CI 环境: hos-sdk (HOS_SDK_HOME)
    
    在 shell 脚本中使用，会从环境变量读取路径
    """
    return """
# HarmonyOS 工具链路径配置
# 支持 Bazel external、DevEco Studio、hos-sdk 三种环境

# ============================================================================
# 检测环境类型
# ============================================================================

USE_BAZEL_EXTERNAL=false
USE_DEVECO=false
USE_HOS_SDK=false

# 1. 检查 Bazel external repository (@hos_sdk)
if [ -n "$HOS_SDK_EXTERNAL" ] && [ -d "$HOS_SDK_EXTERNAL" ]; then
    USE_BAZEL_EXTERNAL=true
fi

# 2. 检查 DevEco Studio (本地开发)
if [ "$USE_BAZEL_EXTERNAL" = false ]; then
    if [ -n "$DEVECO_HOME" ] && [ -d "$DEVECO_HOME" ]; then
        USE_DEVECO=true
    elif [ -d "/Applications/DevEco-Studio.app/Contents" ]; then
        export DEVECO_HOME="/Applications/DevEco-Studio.app/Contents"
        USE_DEVECO=true
    fi
fi

# 3. 检查 hos-sdk (CI 环境)
if [ "$USE_BAZEL_EXTERNAL" = false ] && [ "$USE_DEVECO" = false ]; then
    if [ -n "$HOS_SDK_HOME" ] && [ -d "$HOS_SDK_HOME" ]; then
        USE_HOS_SDK=true
    fi
fi

# 如果都没有，报错
if [ "$USE_BAZEL_EXTERNAL" = false ] && [ "$USE_DEVECO" = false ] && [ "$USE_HOS_SDK" = false ]; then
    echo "ERROR: 未找到 HarmonyOS 工具链"
    echo ""
    echo "方式 1 (推荐): Bazel 会自动下载 @hos_sdk"
    echo "方式 2: 安装 DevEco Studio 或设置 DEVECO_HOME"
    echo "方式 3: 设置 HOS_SDK_HOME 指向 hos-sdk command-line-tools"
    exit 1
fi

# ============================================================================
# 配置工具路径
# ============================================================================

if [ "$USE_BAZEL_EXTERNAL" = true ]; then
    # Bazel external repository 环境
    echo "使用 Bazel @hos_sdk: $HOS_SDK_EXTERNAL"
    
    HOS_SDK_HOME="$HOS_SDK_EXTERNAL"
    SDK_HOME="${HARMONYOS_SDK_HOME:-$HOS_SDK_HOME/sdk}"
    JAVA_HOME="${JAVA_HOME:-$(/usr/libexec/java_home 2>/dev/null || echo /usr/local/opt/openjdk)}"
    NODE_HOME="${NODE_HOME:-$(dirname $(dirname $(which node 2>/dev/null || echo /usr/local/bin/node)))}"
    
    OHPM="$HOS_SDK_HOME/ohpm/bin/ohpm"
    HVIGORW="$(which hvigorw 2>/dev/null || echo $HOS_SDK_HOME/hvigor/bin/hvigorw)"
    
    # 查找 toolchains
    OPENHARMONY_TOOLCHAINS=""
    for api in 12 11 10 9 8; do
        if [ -d "$SDK_HOME/openharmony/$api/toolchains" ]; then
            OPENHARMONY_TOOLCHAINS="$SDK_HOME/openharmony/$api/toolchains"
            break
        fi
    done
    
    HDC="$OPENHARMONY_TOOLCHAINS/hdc"
    TOOLCHAINS_LIB="$OPENHARMONY_TOOLCHAINS/lib"
    
    export PATH="$HOS_SDK_HOME/ohpm/bin:$OPENHARMONY_TOOLCHAINS:$JAVA_HOME/bin:$NODE_HOME/bin:$PATH"

elif [ "$USE_DEVECO" = true ]; then
    # DevEco Studio 环境
    echo "使用 DevEco Studio 工具链: $DEVECO_HOME"
    
    SDK_HOME="${HARMONYOS_SDK_HOME:-$DEVECO_HOME/sdk}"
    JAVA_HOME="${JAVA_HOME:-$DEVECO_HOME/jbr/Contents/Home}"
    NODE_HOME="${NODE_HOME:-$DEVECO_HOME/tools/node}"
    
    HVIGORW="$DEVECO_HOME/tools/hvigor/bin/hvigorw"
    OHPM="$DEVECO_HOME/tools/ohpm/bin/ohpm"
    HDC="$SDK_HOME/default/openharmony/toolchains/hdc"
    
    TOOLCHAINS_LIB="$SDK_HOME/default/openharmony/toolchains/lib"
    
    export PATH="$DEVECO_HOME/tools/hvigor/bin:$DEVECO_HOME/tools/ohpm/bin:$SDK_HOME/default/openharmony/toolchains:$JAVA_HOME/bin:$NODE_HOME/bin:$PATH"
    
elif [ "$USE_HOS_SDK" = true ]; then
    # CI hos-sdk 环境
    echo "使用 hos-sdk 工具链: $HOS_SDK_HOME"
    
    SDK_HOME="${HARMONYOS_SDK_HOME:-$HOS_SDK_HOME/sdk}"
    JAVA_HOME="${JAVA_HOME:-/usr/local/opt/openjdk@17}"
    NODE_HOME="${NODE_HOME:-$(dirname $(dirname $(which node 2>/dev/null || echo /usr/local)))}"
    
    OHPM="$HOS_SDK_HOME/ohpm/bin/ohpm"
    HVIGORW="$(which hvigorw 2>/dev/null || echo hvigorw)"
    
    OPENHARMONY_TOOLCHAINS=""
    for api in 12 11 10 9 8; do
        if [ -d "$SDK_HOME/openharmony/$api/toolchains" ]; then
            OPENHARMONY_TOOLCHAINS="$SDK_HOME/openharmony/$api/toolchains"
            break
        fi
    done
    
    HDC="$OPENHARMONY_TOOLCHAINS/hdc"
    TOOLCHAINS_LIB="$OPENHARMONY_TOOLCHAINS/lib"
    
    export PATH="$HOS_SDK_HOME/bin:$HOS_SDK_HOME/ohpm/bin:$OPENHARMONY_TOOLCHAINS:$JAVA_HOME/bin:$NODE_HOME/bin:$PATH"
fi

# ============================================================================
# 通用变量
# ============================================================================

JAVA="$JAVA_HOME/bin/java"
KEYTOOL="$JAVA_HOME/bin/keytool"
SIGN_TOOL="$TOOLCHAINS_LIB/hap-sign-tool.jar"
PACK_TOOL="$TOOLCHAINS_LIB/app_packing_tool.jar"
KEYSTORE="$TOOLCHAINS_LIB/OpenHarmony.p12"
PROFILE_CERT="$TOOLCHAINS_LIB/OpenHarmonyProfileDebug.pem"
PROFILE_TEMPLATE="$TOOLCHAINS_LIB/UnsgnedDebugProfileTemplate.json"

export NODE_HOME
export JAVA_HOME
export DEVECO_SDK_HOME="$SDK_HOME"

# 验证关键工具
if [ ! -f "$OHPM" ] && ! command -v ohpm &>/dev/null; then
    echo "WARNING: ohpm 未找到"
fi
if [ ! -f "$HVIGORW" ] && ! command -v hvigorw &>/dev/null; then
    echo "WARNING: hvigorw 未找到，请运行: npm install -g @ohos/hvigor"
fi
"""
