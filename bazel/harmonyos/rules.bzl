"""HarmonyOS Bazel Rules 实现

提供构建 HarmonyOS 应用的 Bazel rules。
所有路径通过环境变量配置，不硬编码。
"""

load(":providers.bzl", "HarmonyOSHapInfo", "HarmonyOSSigningInfo")
load(":toolchain.bzl", "get_toolchain_env_script")

# ============================================================================
# harmonyos_hap - 构建 HAP 包
# ============================================================================

def _harmonyos_hap_impl(ctx):
    """构建 HarmonyOS HAP 包"""
    
    # 输出文件
    unsigned_hap = ctx.actions.declare_file(ctx.label.name + "-unsigned.hap")
    
    # 收集所有源文件
    srcs = ctx.files.srcs
    resources = ctx.files.resources
    
    # 构建脚本
    build_script = ctx.actions.declare_file(ctx.label.name + "_build.sh")
    
    # 工具链环境配置（从环境变量获取）
    env_script = get_toolchain_env_script()
    
    script_content = """#!/bin/bash
set -e

{env_script}

# 获取 workspace 根目录
# 在 bazel build 时从环境变量获取，bazel run 时使用 BUILD_WORKSPACE_DIRECTORY
WORKSPACE_ROOT="${{BUILD_WORKSPACE_DIRECTORY:-$PWD}}"

# 如果是相对路径，转换为绝对路径
PROJECT_DIR="{project_dir}"
if [[ "$PROJECT_DIR" != /* ]]; then
    PROJECT_DIR="$WORKSPACE_ROOT/$PROJECT_DIR"
fi

echo "=== HarmonyOS HAP 构建 ==="
echo "Workspace: $WORKSPACE_ROOT"
echo "Project: $PROJECT_DIR"

# 进入项目目录
cd "$PROJECT_DIR"

# 安装依赖
"$OHPM" install

# 构建 HAP
"$HVIGORW" assembleHap --no-daemon -p product={product}

# 复制输出（使用绝对路径）
HAP_FILE=$(find entry/build -name "*.hap" -type f | head -1)
if [ -f "$HAP_FILE" ]; then
    # 输出路径需要转换为绝对路径
    OUTPUT="{output}"
    if [[ "$OUTPUT" != /* ]]; then
        OUTPUT="$WORKSPACE_ROOT/$OUTPUT"
    fi
    mkdir -p "$(dirname "$OUTPUT")"
    cp "$HAP_FILE" "$OUTPUT"
    echo "HAP built successfully: $OUTPUT"
else
    echo "ERROR: HAP file not found"
    exit 1
fi
""".format(
        env_script = env_script,
        project_dir = ctx.attr.project_dir,
        product = ctx.attr.product,
        output = unsigned_hap.path,
    )
    
    ctx.actions.write(
        output = build_script,
        content = script_content,
        is_executable = True,
    )
    
    # 运行构建（需要本地执行，因为要访问 DevEco Studio 工具和项目目录）
    ctx.actions.run(
        inputs = srcs + resources,
        outputs = [unsigned_hap],
        executable = build_script,
        mnemonic = "HarmonyOSHap",
        progress_message = "Building HarmonyOS HAP %s" % ctx.label.name,
        use_default_shell_env = True,
        execution_requirements = {
            "local": "1",  # 本地执行，不使用 sandbox
            "no-cache": "1",  # 不缓存（因为依赖外部工具链）
        },
    )
    
    return [
        DefaultInfo(
            files = depset([unsigned_hap]),
            runfiles = ctx.runfiles(files = [unsigned_hap]),
        ),
        HarmonyOSHapInfo(
            hap = unsigned_hap,
            unsigned_hap = unsigned_hap,
            bundle_name = ctx.attr.bundle_name,
            module_name = ctx.attr.module_name,
            is_signed = False,
        ),
    ]

harmonyos_hap = rule(
    implementation = _harmonyos_hap_impl,
    attrs = {
        "srcs": attr.label_list(
            allow_files = [".ets", ".ts", ".js"],
            doc = "ArkTS/TypeScript 源文件",
        ),
        "resources": attr.label_list(
            allow_files = True,
            doc = "资源文件",
        ),
        "project_dir": attr.string(
            mandatory = True,
            doc = "HarmonyOS 项目目录路径",
        ),
        "bundle_name": attr.string(
            mandatory = True,
            doc = "Bundle 名称 (如 com.example.app)",
        ),
        "module_name": attr.string(
            default = "entry",
            doc = "模块名称",
        ),
        "product": attr.string(
            default = "default",
            doc = "构建产品配置名称",
        ),
    },
    doc = """构建 HarmonyOS HAP 包

需要设置环境变量 DEVECO_HOME 指向 DevEco Studio 安装路径。
可选环境变量：
- HARMONYOS_SDK_HOME: SDK 路径
- JAVA_HOME: Java 路径
""",
)

# ============================================================================
# harmonyos_sign - 签名 HAP 包
# ============================================================================

def _harmonyos_sign_impl(ctx):
    """签名 HarmonyOS HAP 包"""
    
    # 输入 HAP
    input_hap = ctx.file.hap
    
    # 输出签名后的 HAP
    signed_hap = ctx.actions.declare_file(ctx.label.name + ".hap")
    
    # 签名 profile
    signed_profile = ctx.actions.declare_file(ctx.label.name + "_profile.p7b")
    
    # 签名脚本
    sign_script = ctx.actions.declare_file(ctx.label.name + "_sign.sh")
    
    # 工具链环境配置
    env_script = get_toolchain_env_script()
    
    # 签名配置（可选自定义）
    keystore_override = ctx.file.keystore.path if ctx.file.keystore else ""
    keystore_pwd = ctx.attr.keystore_password or "123456"
    key_alias = ctx.attr.key_alias or "openharmony application release"
    key_pwd = ctx.attr.key_password or "123456"
    profile_key_alias = ctx.attr.profile_key_alias or "openharmony application profile debug"
    profile_template_override = ctx.file.profile_template.path if ctx.file.profile_template else ""
    
    script_content = """#!/bin/bash
set -e

{env_script}

# 签名配置
KEYSTORE_OVERRIDE="{keystore_override}"
KEYSTORE_PWD="{keystore_pwd}"
KEY_ALIAS="{key_alias}"
KEY_PWD="{key_pwd}"
PROFILE_KEY_ALIAS="{profile_key_alias}"
PROFILE_TEMPLATE_OVERRIDE="{profile_template_override}"

# 使用自定义或默认配置
if [ -n "$KEYSTORE_OVERRIDE" ]; then
    KEYSTORE="$KEYSTORE_OVERRIDE"
fi
if [ -n "$PROFILE_TEMPLATE_OVERRIDE" ]; then
    PROFILE_TEMPLATE="$PROFILE_TEMPLATE_OVERRIDE"
fi

INPUT_HAP="{input_hap}"
SIGNED_HAP="{signed_hap}"
SIGNED_PROFILE="{signed_profile}"

echo "=== 签名 HarmonyOS HAP ==="
echo "DEVECO_HOME: $DEVECO_HOME"
echo "KEYSTORE: $KEYSTORE"

# 步骤 1: 签名 profile
echo "签名 profile..."
"$JAVA" -jar "$SIGN_TOOL" sign-profile \\
    -keyAlias "$PROFILE_KEY_ALIAS" \\
    -keyPwd "$KEY_PWD" \\
    -signAlg SHA256withECDSA \\
    -mode localSign \\
    -profileCertFile "$PROFILE_CERT" \\
    -inFile "$PROFILE_TEMPLATE" \\
    -keystoreFile "$KEYSTORE" \\
    -keystorePwd "$KEYSTORE_PWD" \\
    -outFile "$SIGNED_PROFILE"

# 步骤 2: 导出证书链
WORK_DIR=$(mktemp -d)
trap "rm -rf $WORK_DIR" EXIT

"$KEYTOOL" -exportcert -alias "$KEY_ALIAS" \\
    -keystore "$KEYSTORE" -storepass "$KEYSTORE_PWD" \\
    -storetype PKCS12 -rfc -file "$WORK_DIR/app.pem"

"$KEYTOOL" -exportcert -alias "openharmony application ca" \\
    -keystore "$KEYSTORE" -storepass "$KEYSTORE_PWD" \\
    -storetype PKCS12 -rfc -file "$WORK_DIR/ca.pem"

"$KEYTOOL" -exportcert -alias "openharmony application root ca" \\
    -keystore "$KEYSTORE" -storepass "$KEYSTORE_PWD" \\
    -storetype PKCS12 -rfc -file "$WORK_DIR/root.pem"

cat "$WORK_DIR/app.pem" "$WORK_DIR/ca.pem" "$WORK_DIR/root.pem" > "$WORK_DIR/cert-chain.pem"

# 步骤 3: 签名 HAP
echo "签名 HAP..."
"$JAVA" -jar "$SIGN_TOOL" sign-app \\
    -mode localSign \\
    -keyAlias "$KEY_ALIAS" \\
    -keyPwd "$KEY_PWD" \\
    -appCertFile "$WORK_DIR/cert-chain.pem" \\
    -profileFile "$SIGNED_PROFILE" \\
    -inFile "$INPUT_HAP" \\
    -signAlg SHA256withECDSA \\
    -keystoreFile "$KEYSTORE" \\
    -keystorePwd "$KEYSTORE_PWD" \\
    -outFile "$SIGNED_HAP" \\
    -signCode "0" \\
    -compatibleVersion 12

echo "签名完成: $SIGNED_HAP"
""".format(
        env_script = env_script,
        keystore_override = keystore_override,
        keystore_pwd = keystore_pwd,
        key_alias = key_alias,
        key_pwd = key_pwd,
        profile_key_alias = profile_key_alias,
        profile_template_override = profile_template_override,
        input_hap = input_hap.path,
        signed_hap = signed_hap.path,
        signed_profile = signed_profile.path,
    )
    
    ctx.actions.write(
        output = sign_script,
        content = script_content,
        is_executable = True,
    )
    
    inputs = [input_hap]
    if ctx.file.keystore:
        inputs.append(ctx.file.keystore)
    if ctx.file.profile_template:
        inputs.append(ctx.file.profile_template)
    
    ctx.actions.run(
        inputs = inputs,
        outputs = [signed_hap, signed_profile],
        executable = sign_script,
        mnemonic = "HarmonyOSSign",
        progress_message = "Signing HarmonyOS HAP %s" % ctx.label.name,
        use_default_shell_env = True,
    )
    
    return [
        DefaultInfo(
            files = depset([signed_hap]),
            runfiles = ctx.runfiles(files = [signed_hap]),
        ),
        HarmonyOSHapInfo(
            hap = signed_hap,
            unsigned_hap = input_hap,
            bundle_name = ctx.attr.bundle_name,
            module_name = ctx.attr.module_name,
            is_signed = True,
        ),
    ]

harmonyos_sign = rule(
    implementation = _harmonyos_sign_impl,
    attrs = {
        "hap": attr.label(
            mandatory = True,
            allow_single_file = [".hap"],
            doc = "要签名的 HAP 文件",
        ),
        "bundle_name": attr.string(
            mandatory = True,
            doc = "Bundle 名称",
        ),
        "module_name": attr.string(
            default = "entry",
            doc = "模块名称",
        ),
        "keystore": attr.label(
            allow_single_file = [".p12", ".jks"],
            doc = "密钥库文件（可选，默认使用 OpenHarmony 调试密钥）",
        ),
        "keystore_password": attr.string(
            doc = "密钥库密码",
        ),
        "key_alias": attr.string(
            doc = "密钥别名",
        ),
        "key_password": attr.string(
            doc = "密钥密码",
        ),
        "profile_key_alias": attr.string(
            doc = "Profile 签名密钥别名",
        ),
        "profile_template": attr.label(
            allow_single_file = [".json"],
            doc = "Profile 模板文件",
        ),
    },
    doc = """签名 HarmonyOS HAP 包

需要设置环境变量 DEVECO_HOME 指向 DevEco Studio 安装路径。
""",
)

# ============================================================================
# harmonyos_install - 安装到设备
# ============================================================================

def _harmonyos_install_impl(ctx):
    """安装 HAP 到设备"""
    
    hap = ctx.file.hap
    
    # 工具链环境配置
    env_script = get_toolchain_env_script()
    
    # 创建安装脚本
    install_script = ctx.actions.declare_file(ctx.label.name + "_install.sh")
    
    script_content = """#!/bin/bash
set -e

{env_script}

HAP="{hap}"
BUNDLE_NAME="{bundle_name}"
ABILITY_NAME="{ability_name}"

echo "=== 安装 HarmonyOS 应用 ==="

# 检查设备连接
DEVICES=$("$HDC" list targets)
if [ -z "$DEVICES" ] || [ "$DEVICES" = "[Empty]" ]; then
    echo "ERROR: 没有连接的设备"
    echo "请启动模拟器或连接设备"
    exit 1
fi
echo "设备: $DEVICES"

# 卸载旧版本（如果存在）
echo "卸载旧版本..."
"$HDC" uninstall "$BUNDLE_NAME" 2>/dev/null || true

# 安装新版本
echo "安装新版本..."
"$HDC" install "$HAP"

# 启动应用
if [ -n "$ABILITY_NAME" ]; then
    echo "启动应用..."
    "$HDC" shell aa start -a "$ABILITY_NAME" -b "$BUNDLE_NAME"
fi

echo "安装完成！"
""".format(
        env_script = env_script,
        hap = hap.short_path,
        bundle_name = ctx.attr.bundle_name,
        ability_name = ctx.attr.ability_name,
    )
    
    ctx.actions.write(
        output = install_script,
        content = script_content,
        is_executable = True,
    )
    
    return [
        DefaultInfo(
            executable = install_script,
            runfiles = ctx.runfiles(files = [hap]),
        ),
    ]

harmonyos_install = rule(
    implementation = _harmonyos_install_impl,
    attrs = {
        "hap": attr.label(
            mandatory = True,
            allow_single_file = [".hap"],
            doc = "要安装的 HAP 文件",
        ),
        "bundle_name": attr.string(
            mandatory = True,
            doc = "Bundle 名称",
        ),
        "ability_name": attr.string(
            default = "EntryAbility",
            doc = "启动的 Ability 名称",
        ),
    },
    executable = True,
    doc = """安装 HarmonyOS 应用到设备

需要设置环境变量 DEVECO_HOME 指向 DevEco Studio 安装路径。
""",
)
