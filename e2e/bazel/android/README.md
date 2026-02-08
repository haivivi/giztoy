# Android Bazel Example

This directory contains Android application examples built with Bazel using `rules_android` and `rules_kotlin`.

## Projects

### 1. HelloWorld (Java)

A traditional Java-based Android application with programmatic UI.

**Files:**
- `java/com/example/helloworld/MainActivity.java` - Main activity
- `res/` - Android resources (strings, styles, colors, icons)
- `AndroidManifest.xml` - Application manifest

### 2. HelloKotlin (Kotlin)

A Kotlin-based Android application demonstrating idiomatic Kotlin code.

**Files:**
- `kotlin/com/example/helloworld/kotlin/MainActivity.kt` - Main activity in Kotlin
- `kotlin/res/` - Kotlin app resources
- `kotlin/AndroidManifest.xml` - Kotlin app manifest

## Building

### Prerequisites

1. Android SDK installed
2. Set `ANDROID_HOME` environment variable:
   ```bash
   export ANDROID_HOME=/path/to/android/sdk
   ```

### Build Commands

```bash
# Build Java app
bazel build //examples/bazel/android:app

# Build Kotlin app
bazel build //examples/bazel/android:kotlin_app

# Build with specific SDK version
bazel build //examples/bazel/android:app --android_sdk=@androidsdk//:sdk
```

## Installing

### On Connected Device/Emulator

```bash
# Install Java app
bazel mobile-install //examples/bazel/android:app

# Install Kotlin app
bazel mobile-install //examples/bazel/android:kotlin_app

# Install with start
bazel mobile-install //examples/bazel/android:app --start_app
```

### Generate APK

The built APK can be found at:
```
bazel-bin/examples/bazel/android/app.apk
bazel-bin/examples/bazel/android/kotlin_app.apk
```

Install manually:
```bash
adb install bazel-bin/examples/bazel/android/app.apk
```

## Configuration

### Android SDK in .bazelrc

Add to your project's `.bazelrc`:

```
# Android SDK configuration
build --android_sdk=@androidsdk//:sdk
build --fat_apk_cpu=armeabi-v7a,arm64-v8a,x86,x86_64
```

### Custom Keystore (for Release Builds)

```python
android_binary(
    name = "app_release",
    keystore = ":release_keystore",
    # ... other settings
)

android_keystore(
    name = "release_keystore",
    keystore = "release.keystore",
    keystore_password = "password",
    key_alias = "key0",
    key_password = "password",
)
```

## Build Rules Reference

### android_library

Compiles Android source files into a library.

```python
android_library(
    name = "mylib",
    srcs = glob(["java/**/*.java"]),
    custom_package = "com.example.myapp",
    manifest = "AndroidManifest.xml",
    resource_files = glob(["res/**"]),
    deps = ["//other:library"],
)
```

### android_binary

Creates an Android APK.

```python
android_binary(
    name = "app",
    custom_package = "com.example.myapp",
    manifest = "AndroidManifest.xml",
    multidex = "native",  # For apps with many methods
    deps = [":mylib"],
)
```

### kt_android_library (Kotlin)

Compiles Kotlin source files for Android.

```python
load("@rules_kotlin//kotlin:android.bzl", "kt_android_library")

kt_android_library(
    name = "kotlin_lib",
    srcs = glob(["kotlin/**/*.kt"]),
    custom_package = "com.example.myapp",
    manifest = "AndroidManifest.xml",
    resource_files = glob(["res/**"]),
)
```

## Common Issues

### SDK Not Found

```
ERROR: no Android SDK found
```

**Solution:** Set `ANDROID_HOME` or configure Android SDK in MODULE.bazel:

```python
android_sdk_repository(
    name = "androidsdk",
    path = "/path/to/android/sdk",
)
```

### Build Tools Missing

```
ERROR: build_tools_version X.Y.Z is not available
```

**Solution:** Install required build tools:
```bash
sdkmanager "build-tools;34.0.0"
```

### ADB Connection Issues

```bash
# List connected devices
adb devices

# Restart ADB server
adb kill-server
adb start-server
```

### Multiple DEX Files Error

For apps exceeding the 64K method limit:

```python
android_binary(
    name = "app",
    multidex = "native",
    # ...
)
```

## ProGuard / R8 Optimization

Enable code shrinking for release builds:

```python
android_binary(
    name = "app_release",
    proguard_specs = ["proguard-rules.pro"],
    shrink_resources = 1,
    # ...
)
```

## Testing

### Unit Tests

```python
android_local_test(
    name = "unit_tests",
    srcs = ["test/MainActivityTest.java"],
    deps = [
        ":lib",
        "@maven//:junit_junit",
    ],
)
```

### Instrumentation Tests

```python
android_instrumentation_test(
    name = "instrumentation_tests",
    target_device = "@android_test_support//tools/android/emulated_devices/generic_phone:android_23_x86_qemu2",
    test_app = ":test_app",
)
```

## Resources

- [rules_android Documentation](https://github.com/bazelbuild/rules_android)
- [rules_kotlin Documentation](https://github.com/bazelbuild/rules_kotlin)
- [Bazel Android Tutorial](https://bazel.build/tutorials/android-app)
- [Android SDK Manager](https://developer.android.com/tools/sdkmanager)
