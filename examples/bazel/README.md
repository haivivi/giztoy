# Bazel Mobile Examples

This directory contains example projects demonstrating how to build iOS and Android applications using Bazel.

## Directory Structure

```
examples/bazel/
├── ios/                    # iOS example using rules_apple
│   ├── HelloWorld/         # UIKit app (Swift)
│   ├── SwiftUIApp/         # SwiftUI app
│   ├── Widget/             # iOS Widget Extension
│   └── BUILD.bazel         # iOS build rules
├── android/                # Android example using rules_android
│   ├── java/               # Java source code
│   ├── kotlin/             # Kotlin source code
│   ├── res/                # Android resources
│   └── BUILD.bazel         # Android build rules
├── harmonyos/              # HarmonyOS example using custom rules
│   ├── HelloWorld/         # ArkTS/ArkUI app
│   ├── rules_harmonyos.bzl # Custom Bazel rules
│   └── BUILD.bazel         # HarmonyOS build rules
└── README.md               # This file
```

## Prerequisites

### iOS Development

- macOS with Xcode installed
- Xcode Command Line Tools: `xcode-select --install`
- A valid Apple Developer account (for device deployment)

### Android Development

- Android SDK installed
- Set `ANDROID_HOME` environment variable or configure in `.bazelrc`

## Building

### Build iOS App

```bash
# Build for simulator (default)
bazel build //examples/bazel/ios:HelloWorld

# Build for device (requires provisioning profile)
bazel build //examples/bazel/ios:HelloWorld --ios_multi_cpus=arm64
```

### Build Android App

```bash
# Build debug APK
bazel build //examples/bazel/android:app

# Build with specific Android SDK
bazel build //examples/bazel/android:app --android_sdk=@androidsdk//:sdk
```

### Build HarmonyOS App

```bash
# Build with custom Bazel rules
bazel build //examples/bazel/harmonyos:HelloWorld

# Build with native hvigorw (requires DevEco Studio)
bazel run //examples/bazel/harmonyos:build_native
```

## Running

### iOS Simulator

```bash
# Install on simulator
bazel run //examples/bazel/ios:HelloWorld
```

### Android Emulator/Device

```bash
# Install on connected device
bazel mobile-install //examples/bazel/android:app
```

## Configuration

### iOS Signing (for device deployment)

Create a `provisioning_profile` file and update the BUILD.bazel with your team ID and bundle identifier.

### Android SDK Path

Add to your `.bazelrc`:

```
build --android_sdk=@androidsdk//:sdk
```

Or set environment variable:

```bash
export ANDROID_HOME=/path/to/android/sdk
```

## Dependencies

These examples require the following Bazel rules (configured in MODULE.bazel):

- **rules_apple** - iOS/macOS/tvOS/watchOS build rules
- **rules_swift** - Swift language support (used by rules_apple)
- **rules_android** - Android build rules
- **rules_harmonyos** - Custom HarmonyOS rules (in `harmonyos/rules_harmonyos.bzl`)

## Troubleshooting

### iOS

1. **Xcode not found**: Ensure Xcode is installed and run `xcode-select -p` to verify path
2. **Signing issues**: Check provisioning profile and team ID settings
3. **Simulator not found**: Run `xcrun simctl list` to see available simulators

### Android

1. **SDK not found**: Verify `ANDROID_HOME` is set correctly
2. **Build tools missing**: Install required build-tools via Android SDK Manager
3. **ADB issues**: Ensure device is connected and USB debugging is enabled
