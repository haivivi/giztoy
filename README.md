# Giztoy

<div align="center">

**A toy framework for building AI-powered applications across all dimensions.**

*From embedded devices to cloud agents, from audio streams to video feeds,*  
*connecting every Large Language Model the universe has to offer.*

[![Build](https://github.com/haivivi/giztoy/actions/workflows/build-main.yaml/badge.svg)](https://github.com/haivivi/giztoy/actions/workflows/build-main.yaml)
[![Docs](https://img.shields.io/github/deployments/haivivi/giztoy/github-pages?label=docs)](https://haivivi.github.io/giztoy/)

[Documentation](https://haivivi.github.io/giztoy/docs/) · [Examples](./examples/) · [中文](./README-zh.md)

</div>

---

## 📚 Documentation

Start here! Preview the documentation locally:

```bash
# Clone and enter the repository
git clone https://github.com/haivivi/giztoy.git
cd giztoy

# Serve documentation locally (requires Bazel)
bazel run //pages:serve-local

# Then open http://localhost:3000/docs/ in your browser
```

Or visit the online documentation: [https://haivivi.github.io/giztoy/docs/](https://haivivi.github.io/giztoy/docs/)

---

## Overview

Giztoy is a multi-language framework designed for building AI toys and intelligent applications. It provides a unified abstraction layer that spans from resource-constrained embedded systems to powerful cloud services.

Think of it as a bridge — not between worlds, but between possibilities.

### Key Features

- **🔌 Embedded First** — Native support for ESP32, nRF, and other MCUs
- **📱 Cross-Platform Apps** — Build for Android, iOS, and HarmonyOS
- **🏗️ Unified Build System** — Bazel compiles everything: mobile apps, MCU firmware, Linux services
- **🤖 Agent Framework** — GenX provides a unified interface for AI agents
- **🎙️ Audio Processing** — Opus, MP3, PCM encoding/decoding with real-time streaming
- **🎬 Video Support** — Coming soon
- **🌐 Universal LLM Support** — OpenAI, Gemini, Claude, MiniMax, DashScope, Doubao, and more
- **⚡ Realtime Models** — WebSocket-based streaming for voice and multimodal AI
- **🔐 Secure Transport** — MQTT for IoT, Noise Protocol + KCP for real-time audio/video
- **🔧 Multi-Language** — Go, Rust, Zig, and C/C++ implementations

### Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    AI Application Layer                      │
│                 GenX · Speech · ChatGear                     │
├─────────────────────────────────────────────────────────────┤
│                     API Client Layer                         │
│    OpenAI · Gemini · Claude · MiniMax · DashScope · Doubao  │
├─────────────────────────────────────────────────────────────┤
│                   Communication Layer                        │
│           MQTT0 · WebSocket · Noise Protocol + KCP          │
├─────────────────────────────────────────────────────────────┤
│                  Audio Processing Layer                      │
│              Opus · MP3 · PCM · Resampler                   │
├─────────────────────────────────────────────────────────────┤
│                    Foundation Layer                          │
│            Buffer · Encoding · Trie · CLI                    │
└─────────────────────────────────────────────────────────────┘
```

### Quick Start

```bash
# Clone the repository
git clone https://github.com/haivivi/giztoy.git
cd giztoy

# Build with Bazel
bazel build //...

# Or use native toolchains
cd go && go build ./cmd/...
cd rust && cargo build --release
```

### Supported Platforms

All platforms built with **Bazel** — one build system to rule them all.

| Platform | Status | Notes |
|----------|--------|-------|
| Linux | ✅ | Full support |
| macOS | ✅ | Full support |
| Android | ✅ | Bazel + rules_android |
| iOS | ✅ | Bazel + rules_apple |
| HarmonyOS | ✅ | Bazel + custom rules |
| ESP32 | 🚧 | Bazel + esp-idf |
| nRF / BLE MCUs | 🚧 | Coming soon |
| Other Linux-based | ✅ | OpenWrt, Yocto, etc. |

### Why Go + Rust + Zig?

Go for simplicity, Rust for performance, Zig for the edge. Each has its strengths.

- **Go** — Rapid prototyping, CLI tools, server applications
- **Rust** — Embedded systems, performance-critical audio/video processing, reliability
- **Zig** — Coming soon, for bare-metal and extreme resource constraints

In Giztoy, nearly every module provides multiple language implementations. The choice is yours.

### Documentation

Full documentation: [https://haivivi.github.io/giztoy/docs/](https://haivivi.github.io/giztoy/docs/)

### License

[Apache License 2.0](./LICENSE)

---

<div align="center">

*"I'm just a toymaker."*

</div>
