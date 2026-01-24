# iOS Bazel Example

This directory contains iOS application examples built with Bazel using `rules_apple` and `rules_swift`.

## Projects

### 1. HelloWorld (UIKit)

A traditional UIKit-based iOS application with programmatic UI.

**Files:**
- `HelloWorld/AppDelegate.swift` - Application lifecycle
- `HelloWorld/SceneDelegate.swift` - Scene lifecycle
- `HelloWorld/ViewController.swift` - Main view controller
- `HelloWorld/Info.plist` - Application configuration

### 2. SwiftUIApp

A modern SwiftUI application with a beautiful gradient UI.

**Files:**
- `SwiftUIApp/App.swift` - SwiftUI App entry point
- `SwiftUIApp/ContentView.swift` - Main content view
- `SwiftUIApp/Info.plist` - Application configuration

### 3. BazelWidget (iOS Widget Extension)

A WidgetKit-based iOS Widget that displays Bazel build status. Supports small, medium, and large widget sizes.

**Files:**
- `Widget/BazelWidget.swift` - Widget implementation with timeline provider and views
- `Widget/Info.plist` - Widget extension configuration

**Features:**
- 📊 Small widget: Build count and status indicator
- 📈 Medium widget: Build stats with last build time
- 📋 Large widget: Full dashboard with recent builds list

**Build Targets:**
- `BazelWidgetExtension` - Standalone widget extension
- `SwiftUIAppWithWidget` - SwiftUI app with embedded widget

## Building

### Prerequisites

1. macOS with Xcode 14+ installed
2. Xcode Command Line Tools:
   ```bash
   xcode-select --install
   ```

### Build Commands

```bash
# Build UIKit app
bazel build //examples/bazel/ios:HelloWorld

# Build SwiftUI app
bazel build //examples/bazel/ios:SwiftUIApp

# Build Widget Extension (standalone)
bazel build //examples/bazel/ios:BazelWidgetExtension

# Build SwiftUI app with Widget embedded
bazel build //examples/bazel/ios:SwiftUIAppWithWidget

# Build for specific architecture
bazel build //examples/bazel/ios:HelloWorld --ios_multi_cpus=arm64

# Build for simulator
bazel build //examples/bazel/ios:HelloWorld --ios_multi_cpus=sim_arm64
```

## Running on Simulator

```bash
# Run UIKit app
bazel run //examples/bazel/ios:HelloWorld

# Run SwiftUI app
bazel run //examples/bazel/ios:SwiftUIApp
```

## Device Deployment

For device deployment, you need to configure code signing:

### 1. Create Provisioning Profile

Add a provisioning profile rule to BUILD.bazel:

```python
provisioning_profile(
    name = "profile",
    profile = "YourProfile.mobileprovision",
)
```

### 2. Update ios_application

```python
ios_application(
    name = "HelloWorld",
    bundle_id = "com.yourcompany.helloworld",
    provisioning_profile = ":profile",
    # ... other settings
)
```

### 3. Build with Signing

```bash
bazel build //examples/bazel/ios:HelloWorld \
    --ios_signing_cert_name="iPhone Developer: Your Name"
```

## Build Rules Reference

### swift_library

Compiles Swift source files into a library.

```python
swift_library(
    name = "MyLib",
    srcs = ["Source.swift"],
    module_name = "MyModule",
    deps = ["//other:library"],
)
```

### ios_application

Creates an iOS application bundle (.app).

```python
ios_application(
    name = "MyApp",
    bundle_id = "com.example.myapp",
    families = ["iphone", "ipad"],
    infoplists = ["Info.plist"],
    minimum_os_version = "15.0",
    deps = [":MyLib"],
)
```

### ios_extension

Creates an iOS extension (Widget, Share Extension, etc.).

```python
ios_extension(
    name = "MyWidget",
    bundle_id = "com.example.myapp.widget",
    families = ["iphone", "ipad"],
    infoplists = ["Widget/Info.plist"],
    minimum_os_version = "17.0",
    deps = [":MyWidgetLib"],
)
```

### Embedding Extensions in App

```python
ios_application(
    name = "MyAppWithWidget",
    bundle_id = "com.example.myapp",
    extensions = [":MyWidget"],  # Embed the widget extension
    families = ["iphone", "ipad"],
    infoplists = ["Info.plist"],
    minimum_os_version = "17.0",
    deps = [":MyAppLib"],
)
```

## Common Issues

### Xcode Version Mismatch

If you get Xcode-related errors:

```bash
# Check current Xcode
xcode-select -p

# Switch Xcode version
sudo xcode-select -s /Applications/Xcode.app
```

### Simulator Not Found

List available simulators:

```bash
xcrun simctl list devices
```

### Swift Version

Specify Swift version in BUILD.bazel if needed:

```python
swift_library(
    name = "MyLib",
    srcs = ["Source.swift"],
    swift_version = "5",
)
```

## Resources

- [rules_apple Documentation](https://github.com/bazelbuild/rules_apple)
- [rules_swift Documentation](https://github.com/bazelbuild/rules_swift)
- [Bazel iOS Tutorial](https://bazel.build/tutorials/ios-app)
