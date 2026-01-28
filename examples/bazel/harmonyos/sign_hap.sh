#!/bin/bash
# HarmonyOS HAP 签名脚本
# 使用 DevEco Studio 自带的工具链签名 HAP 包

set -e

# Check if running inside bazel
if [[ -z "$BUILD_WORKSPACE_DIRECTORY" ]]; then
    echo "ERROR: This script must be run via bazel." >&2
    echo >&2
    echo "Usage:" >&2
    echo "  bazel run //examples/bazel/harmonyos:sign_hap -- <unsigned.hap> <signed.hap> [profile.json]" >&2
    echo >&2
    echo "Example:" >&2
    echo "  bazel run //examples/bazel/harmonyos:sign_hap -- input.hap output.hap" >&2
    exit 1
fi

# DevEco 工具路径
DEVECO_HOME="/Applications/DevEco-Studio.app/Contents"
JAVA="$DEVECO_HOME/jbr/Contents/Home/bin/java"
TOOLCHAINS="$DEVECO_HOME/sdk/default/openharmony/toolchains"
HAP_SIGN_TOOL="$TOOLCHAINS/lib/hap-sign-tool.jar"
KEYSTORE="$TOOLCHAINS/lib/OpenHarmony.p12"
KEYSTORE_PWD="123456"
KEY_ALIAS="openharmony application release"
PROFILE_KEY_ALIAS="openharmony application profile debug"

# 输入输出
UNSIGNED_HAP="${1:-}"
SIGNED_HAP="${2:-}"
PROFILE_JSON="${3:-$TOOLCHAINS/lib/UnsgnedDebugProfileTemplate.json}"

if [ -z "$UNSIGNED_HAP" ] || [ -z "$SIGNED_HAP" ]; then
    echo "用法: $0 <未签名HAP> <签名后HAP> [profile.json]"
    echo ""
    echo "示例:"
    echo "  $0 input.hap output.hap"
    exit 1
fi

WORK_DIR=$(mktemp -d)
trap "rm -rf $WORK_DIR" EXIT

echo "=== HarmonyOS HAP 签名 ==="
echo "输入: $UNSIGNED_HAP"
echo "输出: $SIGNED_HAP"
echo ""

# 步骤 1: 生成签名的 profile (p7b)
echo "步骤 1: 生成签名的 profile..."

# 修改 profile JSON 中的 bundle-name
PROFILE_TMP="$WORK_DIR/profile.json"
cat "$PROFILE_JSON" | sed 's/"bundle-name": "[^"]*"/"bundle-name": "com.example.hellobazel"/' > "$PROFILE_TMP"

SIGNED_PROFILE="$WORK_DIR/signed-profile.p7b"

"$JAVA" -jar "$HAP_SIGN_TOOL" sign-profile \
    -keyAlias "$PROFILE_KEY_ALIAS" \
    -keyPwd "$KEYSTORE_PWD" \
    -signAlg SHA256withECDSA \
    -mode localSign \
    -profileCertFile "$TOOLCHAINS/lib/OpenHarmonyProfileDebug.pem" \
    -inFile "$PROFILE_TMP" \
    -keystoreFile "$KEYSTORE" \
    -keystorePwd "$KEYSTORE_PWD" \
    -outFile "$SIGNED_PROFILE" 2>&1 || {
    echo "错误: profile 签名失败"
    exit 1
}

# 步骤 2: 签名 HAP
echo ""
echo "步骤 2: 签名 HAP..."

# 提取应用证书
APP_CERT="$WORK_DIR/app-cert.pem"
cat > "$APP_CERT" << 'CERT'
-----BEGIN CERTIFICATE-----
MIICMzCCAbegAwIBAgIEaOC/zDAMBggqhkjOPQQDAwUAMGMxCzAJBgNVBAYTAkNO
MRQwEgYDVQQKEwtPcGVuSGFybW9ueTEZMBcGA1UECxMQT3Blbkhhcm1vbnkgVGVh
bTEjMCEGA1UEAxMaT3Blbkhhcm1vbnkgQXBwbGljYXRpb24gQ0EwHhcNMjEwMjAy
MTIxOTMxWhcNNDkxMjMxMTIxOTMxWjBoMQswCQYDVQQGEwJDTjEUMBIGA1UEChML
T3Blbkhhcm1vbnkxGTAXBgNVBAsTEE9wZW5IYXJtb255IFRlYW0xKDAmBgNVBAMT
H09wZW5IYXJtb255IEFwcGxpY2F0aW9uIFJlbGVhc2UwWTATBgcqhkjOPQIBBggq
hkjOPQMBBwNCAATbYOCQQpW5fdkYHN45v0X3AHax12jPBdEDosFRIZ1eXmxOYzSG
JwMfsHhUU90E8lI0TXYZnNmgM1sovubeQqATo1IwUDAfBgNVHSMEGDAWgBTbhrci
FtULoUu33SV7ufEFfaItRzAOBgNVHQ8BAf8EBAMCB4AwHQYDVR0OBBYEFPtxruhl
cRBQsJdwcZqLu9oNUVgaMAwGCCqGSM49BAMDBQADaAAwZQIxAJta0PQ2p4DIu/ps
LMdLCDgQ5UH1l0B4PGhBlMgdi2zf8nk9spazEQI/0XNwpft8QAIwHSuA2WelVi/o
zAlF08DnbJrOOtOnQq5wHOPlDYB4OtUzOYJk9scotrEnJxJzGsh/
-----END CERTIFICATE-----
CERT

"$JAVA" -jar "$HAP_SIGN_TOOL" sign-app \
    -mode localSign \
    -keyAlias "$KEY_ALIAS" \
    -keyPwd "$KEYSTORE_PWD" \
    -appCertFile "$APP_CERT" \
    -profileFile "$SIGNED_PROFILE" \
    -inFile "$UNSIGNED_HAP" \
    -signAlg SHA256withECDSA \
    -keystoreFile "$KEYSTORE" \
    -keystorePwd "$KEYSTORE_PWD" \
    -outFile "$SIGNED_HAP" \
    -signCode "0" \
    -compatibleVersion 9 2>&1

echo ""
echo "=== 签名完成 ==="
ls -la "$SIGNED_HAP"
