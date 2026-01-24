"""Custom Bazel rules for HarmonyOS application development.

These rules wrap the HarmonyOS toolchain (hvigorw) to enable building
HarmonyOS applications with Bazel.

Requirements:
- DevEco Studio installed (provides hvigorw)
- DEVECO_SDK_HOME environment variable set
- Node.js installed (required by hvigor)
"""

def _harmonyos_project_impl(ctx):
    """Implementation of harmonyos_project rule."""
    
    # Create output directory
    out_dir = ctx.actions.declare_directory(ctx.label.name + "_out")
    
    # Create build script
    build_script = ctx.actions.declare_file(ctx.label.name + "_build.sh")
    
    script_content = """#!/bin/bash
set -e

PROJECT_DIR="{project_dir}"
OUT_DIR="{out_dir}"
BUILD_MODE="{build_mode}"

# Check if hvigorw exists
if [ -z "$DEVECO_SDK_HOME" ]; then
    echo "ERROR: DEVECO_SDK_HOME environment variable not set"
    echo "Please install DevEco Studio and set DEVECO_SDK_HOME"
    exit 1
fi

HVIGOR="$DEVECO_SDK_HOME/tools/hvigor/bin/hvigorw"
if [ ! -f "$HVIGOR" ]; then
    # Try alternative path
    HVIGOR=$(which hvigorw 2>/dev/null || echo "")
    if [ -z "$HVIGOR" ]; then
        echo "ERROR: hvigorw not found"
        echo "Please ensure DevEco Studio is installed and hvigorw is in PATH"
        exit 1
    fi
fi

cd "$PROJECT_DIR"

# Run hvigor build
echo "Building HarmonyOS project with hvigorw..."
"$HVIGOR" assembleHap -p product=default -p buildMode="$BUILD_MODE" --no-daemon

# Copy output to Bazel output directory
mkdir -p "$OUT_DIR"
find . -name "*.hap" -exec cp {{}} "$OUT_DIR/" \\;

echo "Build completed. Output in $OUT_DIR"
""".format(
        project_dir = ctx.file.project_dir.path if ctx.file.project_dir else ".",
        out_dir = out_dir.path,
        build_mode = ctx.attr.build_mode,
    )
    
    ctx.actions.write(
        output = build_script,
        content = script_content,
        is_executable = True,
    )
    
    # Collect all source files
    srcs = ctx.files.srcs
    
    ctx.actions.run(
        outputs = [out_dir],
        inputs = srcs + ([ctx.file.project_dir] if ctx.file.project_dir else []),
        executable = build_script,
        use_default_shell_env = True,
        mnemonic = "HarmonyOSBuild",
        progress_message = "Building HarmonyOS project %s" % ctx.label.name,
    )
    
    return [DefaultInfo(files = depset([out_dir]))]

harmonyos_project = rule(
    implementation = _harmonyos_project_impl,
    attrs = {
        "srcs": attr.label_list(
            allow_files = True,
            doc = "Source files for the HarmonyOS project",
        ),
        "project_dir": attr.label(
            allow_single_file = True,
            doc = "Root directory of the HarmonyOS project",
        ),
        "build_mode": attr.string(
            default = "debug",
            values = ["debug", "release"],
            doc = "Build mode: debug or release",
        ),
        "module_name": attr.string(
            default = "entry",
            doc = "Name of the module to build",
        ),
    },
    doc = "Builds a HarmonyOS project using hvigorw.",
)

def _harmonyos_hap_impl(ctx):
    """Implementation of harmonyos_hap rule for building HAP packages."""
    
    out_hap = ctx.actions.declare_file(ctx.label.name + ".hap")
    
    # Create a wrapper script that handles the build
    build_script = ctx.actions.declare_file(ctx.label.name + "_hap_build.sh")
    
    # Build copy commands for ETS files
    copy_commands = []
    for f in ctx.files.srcs:
        if f.path.endswith(".ets"):
            copy_commands.append('cp "$PWD/{}" "$TEMP_DIR/ets/" 2>/dev/null || true'.format(f.path))
    
    script_content = """#!/bin/bash
set -e

echo "=== HarmonyOS HAP Build ==="
echo "Module: {module_name}"
echo "Bundle ID: {bundle_id}"

# Save current directory
ORIG_DIR="$PWD"
OUT_HAP="$ORIG_DIR/{out_hap}"

# Create output directory
mkdir -p "$(dirname "$OUT_HAP")"

# Create a minimal HAP structure (ZIP format)
TEMP_DIR=$(mktemp -d)
trap "rm -rf $TEMP_DIR" EXIT

mkdir -p "$TEMP_DIR/ets"
mkdir -p "$TEMP_DIR/resources"

# Copy source files
cd "$ORIG_DIR"
{copy_commands}

# Create module.json
cat > "$TEMP_DIR/module.json" << 'MODULEEOF'
{{
  "module": {{
    "name": "{module_name}",
    "type": "entry",
    "description": "HarmonyOS Entry Module",
    "mainElement": "EntryAbility",
    "deviceTypes": ["phone", "tablet"],
    "deliveryWithInstall": true,
    "installationFree": false
  }}
}}
MODULEEOF

# Package as HAP (ZIP format)
cd "$TEMP_DIR"
zip -r "$OUT_HAP" . > /dev/null

echo "HAP package created: $OUT_HAP"
""".format(
        module_name = ctx.attr.module_name,
        bundle_id = ctx.attr.bundle_id,
        out_hap = out_hap.path,
        copy_commands = "\n".join(copy_commands) if copy_commands else "echo 'No ETS files to copy'",
    )
    
    ctx.actions.write(
        output = build_script,
        content = script_content,
        is_executable = True,
    )
    
    ctx.actions.run(
        outputs = [out_hap],
        inputs = ctx.files.srcs,
        executable = build_script,
        use_default_shell_env = True,
        mnemonic = "HarmonyOSHAP",
        progress_message = "Building HAP %s" % ctx.label.name,
    )
    
    return [DefaultInfo(files = depset([out_hap]))]

harmonyos_hap = rule(
    implementation = _harmonyos_hap_impl,
    attrs = {
        "srcs": attr.label_list(
            allow_files = [".ets", ".ts", ".json", ".json5"],
            mandatory = True,
            doc = "ArkTS/TypeScript source files",
        ),
        "resources": attr.label_list(
            allow_files = True,
            doc = "Resource files (images, strings, etc.)",
        ),
        "module_name": attr.string(
            default = "entry",
            doc = "Module name",
        ),
        "bundle_id": attr.string(
            mandatory = True,
            doc = "Bundle identifier (e.g., com.example.myapp)",
        ),
        "min_api_version": attr.int(
            default = 11,
            doc = "Minimum API version",
        ),
        "target_api_version": attr.int(
            default = 12,
            doc = "Target API version",
        ),
        "deps": attr.label_list(
            doc = "Dependencies",
        ),
    },
    doc = "Builds a HarmonyOS HAP package.",
)

def _harmonyos_app_impl(ctx):
    """Implementation of harmonyos_app rule for building APP packages."""
    
    out_app = ctx.actions.declare_file(ctx.label.name + ".app")
    
    build_script = ctx.actions.declare_file(ctx.label.name + "_app_build.sh")
    
    # Collect HAP files from dependencies
    hap_files = []
    for dep in ctx.attr.haps:
        for f in dep.files.to_list():
            if f.path.endswith(".hap"):
                hap_files.append(f)
    
    script_content = """#!/bin/bash
set -e

echo "=== HarmonyOS APP Build ==="
echo "App Name: {app_name}"
echo "Bundle ID: {bundle_id}"

# Save current directory
ORIG_DIR="$PWD"
OUT_APP="$ORIG_DIR/{out_app}"

# Create output directory
mkdir -p "$(dirname "$OUT_APP")"

TEMP_DIR=$(mktemp -d)
trap "rm -rf $TEMP_DIR" EXIT

mkdir -p "$TEMP_DIR"

# Copy HAP files
cd "$ORIG_DIR"
{copy_haps}

# Create pack.info
cat > "$TEMP_DIR/pack.info" << 'PACKEOF'
{{
  "summary": {{
    "app": {{
      "bundleName": "{bundle_id}",
      "bundleType": "app",
      "version": {{
        "code": 1000000,
        "name": "1.0.0"
      }}
    }},
    "modules": []
  }},
  "packages": []
}}
PACKEOF

# Package as APP
cd "$TEMP_DIR"
zip -r "$OUT_APP" . > /dev/null

echo "APP package created: $OUT_APP"
""".format(
        app_name = ctx.attr.app_name,
        bundle_id = ctx.attr.bundle_id,
        out_app = out_app.path,
        copy_haps = "\n".join([
            'cp "$ORIG_DIR/{}" "$TEMP_DIR/"'.format(f.path) for f in hap_files
        ]) if hap_files else "echo 'No HAP files to copy'",
    )
    
    ctx.actions.write(
        output = build_script,
        content = script_content,
        is_executable = True,
    )
    
    ctx.actions.run(
        outputs = [out_app],
        inputs = hap_files,
        executable = build_script,
        use_default_shell_env = True,
        mnemonic = "HarmonyOSAPP",
        progress_message = "Building APP %s" % ctx.label.name,
    )
    
    return [DefaultInfo(files = depset([out_app]))]

harmonyos_app = rule(
    implementation = _harmonyos_app_impl,
    attrs = {
        "haps": attr.label_list(
            mandatory = True,
            doc = "HAP modules to include in the APP",
        ),
        "app_name": attr.string(
            mandatory = True,
            doc = "Application name",
        ),
        "bundle_id": attr.string(
            mandatory = True,
            doc = "Bundle identifier",
        ),
        "signing_config": attr.label(
            allow_single_file = True,
            doc = "Signing configuration file",
        ),
    },
    doc = "Builds a HarmonyOS APP package from HAP modules.",
)

# Convenience macro for creating a complete HarmonyOS application
def harmonyos_application(
        name,
        srcs,
        bundle_id,
        resources = [],
        module_name = "entry",
        app_name = None,
        **kwargs):
    """Creates a complete HarmonyOS application with HAP and APP packages.
    
    Args:
        name: Target name
        srcs: ArkTS source files
        bundle_id: Bundle identifier
        resources: Resource files
        module_name: Module name (default: entry)
        app_name: Application display name (default: same as name)
        **kwargs: Additional arguments passed to rules
    """
    
    hap_name = name + "_hap"
    
    harmonyos_hap(
        name = hap_name,
        srcs = srcs,
        resources = resources,
        module_name = module_name,
        bundle_id = bundle_id,
        **kwargs
    )
    
    harmonyos_app(
        name = name,
        haps = [":" + hap_name],
        app_name = app_name or name,
        bundle_id = bundle_id,
    )
