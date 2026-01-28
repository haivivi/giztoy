# 智子玩具

<div align="center">

**为人类准备的 AI 玩具框架**

*从嵌入式设备到云端智能体，从音频流到视频流，*  
*连接宇宙中所有的大语言模型。*

[![Build](https://github.com/haivivi/giztoy/actions/workflows/build-main.yaml/badge.svg)](https://github.com/haivivi/giztoy/actions/workflows/build-main.yaml)
[![Docs](https://img.shields.io/github/deployments/haivivi/giztoy/github-pages?label=docs)](https://haivivi.github.io/giztoy/)

[文档](https://haivivi.github.io/giztoy/docs/zh/) · [示例](./examples/) · [English](./README.md)

</div>

---

## 📚 文档

从这里开始！本地预览文档：

```bash
# 克隆并进入仓库
git clone https://github.com/haivivi/giztoy.git
cd giztoy

# 本地启动文档服务（需要 Bazel）
bazel run //pages:serve-local

# 然后在浏览器打开 http://localhost:3000/docs/zh/
```

或访问在线文档：[https://haivivi.github.io/giztoy/docs/zh/](https://haivivi.github.io/giztoy/docs/zh/)

---

## 写在前面

> *「在足够长的时间尺度上，任何技术都将变得像玩具一样简单。」*

这个项目的名字叫 Giztoy，中文名「智子玩具」。

为什么叫玩具？因为对于真正理解技术本质的存在而言，即便是最复杂的系统，也不过是宇宙中的一个小把戏。而我们要做的，就是把这些「小把戏」变得足够简单，简单到任何人都能拿起来玩。

这个框架的设计理念很简单：**消除边界**。

嵌入式设备与云端服务之间的边界，Go 与 Rust 之间的边界，不同大模型厂商之间的边界，音频与视频之间的边界，人与 AI 之间的边界——这些人为制造的隔阂，在更高的视角看来，本就不应该存在。

---

## 核心能力

### 🔮 全维度覆盖

从一颗小小的 ESP32 芯片，到运行在云端的智能体；从 Android 手机，到 iOS 平板，再到鸿蒙设备——Giztoy 让你的代码在任何地方运行。

### 🏗️ 统一构建系统

整个项目构建于 **Bazel** 之上。一套构建系统，编译一切：

- 📱 移动应用：Android、iOS、HarmonyOS
- 🔌 嵌入式固件：ESP32、nRF、各类 MCU
- 🖥️ Linux 系统：桌面、服务器、OpenWrt、Yocto

不再需要为每个平台维护不同的构建脚本。Bazel 统一了这一切。

### 🎭 大模型统一接口

OpenAI、Gemini、Claude、MiniMax、通义千问（DashScope）、豆包（Doubao）、九天……还有更多。GenX 模块提供了一个统一的抽象层，让你可以像切换玩具一样切换不同的 AI 大脑。

### 🔐 安全传输协议

两代通信协议并行：
- **MQTT0** —— 第一代，适用于 IoT 场景的轻量级消息传输
- **Noise Protocol + KCP** —— 第二代，基于 Noise Protocol 的端到端加密，结合 KCP 实现低延迟音视频传输

### 🎵 实时音视频

Opus 编解码、MP3 流式处理、PCM 采样率转换——所有你需要的音频能力，都在这里。视频支持？正在路上。

### ⚡ Realtime 支持

支持 OpenAI Realtime API 以及各家厂商的实时语音模型。毫秒级延迟，让 AI 真正「活」起来。

---

## 架构

```
┌─────────────────────────────────────────────────────────────┐
│                      AI 应用层                               │
│                 GenX · Speech · ChatGear                     │
├─────────────────────────────────────────────────────────────┤
│                     API 客户端层                             │
│    OpenAI · Gemini · Claude · MiniMax · DashScope · Doubao  │
├─────────────────────────────────────────────────────────────┤
│                       通信层                                 │
│           MQTT0 · WebSocket · Noise Protocol + KCP          │
├─────────────────────────────────────────────────────────────┤
│                     音频处理层                               │
│              Opus · MP3 · PCM · Resampler                   │
├─────────────────────────────────────────────────────────────┤
│                       基础层                                 │
│            Buffer · Encoding · Trie · CLI                    │
└─────────────────────────────────────────────────────────────┘
```

---

## 目录结构

```
giztoy/
├── go/                 # Go 实现
│   ├── cmd/            # CLI 工具
│   └── pkg/            # 核心库
├── rust/               # Rust 实现
│   ├── cmd/            # CLI 工具
│   └── */              # 各个 crate
├── esp/                # ESP32 嵌入式代码
├── examples/           # 示例代码
├── docs/               # 文档
└── pages/              # 文档网站
```

---

## 开始使用

```bash
# 克隆仓库
git clone https://github.com/haivivi/giztoy.git
cd giztoy

# Bazel 构建（推荐）
bazel build //...
bazel test //...

# 或者使用原生工具链
cd go && go build ./cmd/...      # Go
cd rust && cargo build --release  # Rust
```

---

## 平台支持

所有平台统一使用 **Bazel** 构建——一个构建系统，统治一切。

| 平台 | 状态 | 备注 |
|------|------|------|
| Linux | ✅ | 完整支持 |
| macOS | ✅ | 完整支持 |
| Android | ✅ | Bazel + rules_android |
| iOS | ✅ | Bazel + rules_apple |
| HarmonyOS | ✅ | Bazel + 自定义规则 |
| ESP32 | 🚧 | Bazel + esp-idf |
| nRF / BLE MCU | 🚧 | 即将支持 |
| 其他 Linux 系统 | ✅ | OpenWrt、Yocto 等 |

---

## 为什么是 Go + Rust + Zig？

Go 的简洁、Rust 的性能、Zig 的极致，各有所长。

- **Go** —— 适合快速原型开发、CLI 工具、服务端应用
- **Rust** —— 适合嵌入式、性能敏感的音视频处理、需要极致可靠性的场景
- **Zig** —— 即将支持，用于裸金属和极端资源受限的场景

在 Giztoy 中，几乎每个模块都提供多语言实现。选择权在你手中。

---

## 文档

完整文档请访问：[https://haivivi.github.io/giztoy/docs/](https://haivivi.github.io/giztoy/docs/)

---

## 许可证

[Apache License 2.0](./LICENSE)

---

<div align="center">

*「我只是个玩具匠人。」*

</div>
