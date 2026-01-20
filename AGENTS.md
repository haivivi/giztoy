# AGENTS.md - Project Design Document

## Project Overview

giztoy is a Bazel-based multi-language project supporting the following languages:
- Go
- Rust
- C/C++

## Core Design Principles

1. **Language Isolation**: All Go code resides in `go/`, all Rust code resides in `rust/`
2. **Independent Examples**: Each example in `examples/` is an independent package demonstrating how to reference project libraries
3. **Dual Build Support**: Supports both native toolchains (go build, cargo) and Bazel builds

## Directory Structure

```
giztoy/
├── MODULE.bazel              # Bazel module definition (bzlmod)
├── BUILD.bazel               # Root BUILD file
├── .bazelrc                  # Bazel configuration
├── .bazelversion             # Bazel version lock
├── .github/
│   └── workflows/
│       └── build.yaml        # GitHub Actions CI
├── README.md
│
├── docs/                     # API documentation
│   ├── dashscope/            # DashScope API docs
│   ├── doubaospeech/         # Doubao Speech API docs
│   └── minimax/              # MiniMax API docs
│
├── go/                       # Go code root directory
│   ├── go.mod                # Go module definition (github.com/haivivi/giztoy)
│   ├── go.sum                # Go dependency checksum
│   ├── cmd/                  # CLI executables
│   │   ├── dashscope/        # DashScope CLI
│   │   ├── doubao/           # Doubao Speech CLI
│   │   └── minimax/          # MiniMax CLI
│   └── pkg/                  # Public libraries
│       ├── audio/            # Audio processing library
│       ├── buffer/           # Buffer utilities
│       ├── cli/              # CLI common utilities
│       ├── dashscope/        # DashScope SDK
│       ├── doubaospeech/     # Doubao Speech SDK
│       ├── genx/             # GenX (LLM universal interface)
│       └── minimax/          # MiniMax SDK
│
├── rust/                     # Rust code root directory
│   ├── Cargo.toml            # Rust workspace definition
│   ├── Cargo.lock            # Rust dependency lock
│   ├── cli/                  # CLI common library
│   ├── minimax/              # MiniMax SDK
│   └── cmd/
│       └── minimax/          # MiniMax CLI
│
├── examples/                 # Example code (independent packages)
│   ├── audio/                # Audio processing examples
│   │   ├── pcm/
│   │   │   ├── mixer/        # PCM mixer example (Go, independent go.mod)
│   │   │   └── resampler/    # Resampler example (Go, independent go.mod)
│   │   └── songs/            # Music playback example (Go, independent go.mod)
│   ├── dashscope/
│   │   └── realtime/         # DashScope realtime conversation example (Go, independent go.mod)
│   ├── doubaospeech/
│   │   ├── cmd/              # CLI test scripts (shell)
│   │   ├── asr_file/         # ASR file recognition example (Go, independent go.mod)
│   │   ├── tts_v3/           # TTS V3 example (Go, independent go.mod)
│   │   └── ...               # Other examples
│   ├── genx/
│   │   └── chat/             # GenX chat example (Go, independent go.mod)
│   └── minimax/
│       ├── cmd/              # CLI test scripts (shell)
│       ├── image_gen/        # Image generation example (Go, independent go.mod)
│       ├── speech_tts/       # TTS example (Go, independent go.mod)
│       └── ...               # Other examples
│
└── third_party/              # Third-party C library configurations
    ├── portaudio/
    └── soxr/
```

## Go Directory Description

| Directory | Purpose |
|-----------|---------|
| `go/cmd/` | CLI executables, each subdirectory is an independent command-line tool |
| `go/pkg/` | Public libraries, can be referenced internally and externally, import path: `github.com/haivivi/giztoy/pkg/...` |

## Rust Directory Description

| Directory | Purpose |
|-----------|---------|
| `rust/cmd/` | CLI executables |
| `rust/minimax/` | MiniMax SDK crate |
| `rust/cli/` | CLI common library crate |

## Examples Design

Each example is an **independent Go module** with its own `go.mod` file, referencing the local `go/` module via the `replace` directive:

```go
// examples/minimax/speech_tts/go.mod
module github.com/haivivi/giztoy/examples/minimax/speech_tts

go 1.25

require github.com/haivivi/giztoy v0.0.0

replace github.com/haivivi/giztoy => ../../../go
```

Benefits of this design:
1. **Demonstrates Real Usage**: Shows how external users would reference packages from this project
2. **Independent Compilation**: Each example can be built separately with `go build`
3. **Dependency Isolation**: Examples can have their own additional dependencies without polluting the main module

### CLI Test Directory (cmd/)

`examples/*/cmd/` directories contain CLI tool integration test scripts:
- `run.sh` - Test script
- `*.yaml` - Test configuration files
- `BUILD.bazel` - Bazel build rules

Running methods:
```bash
# Direct execution
./examples/minimax/cmd/run.sh go 1

# Bazel execution
bazel run //examples/minimax/cmd:run -- go 1
```

## Bazel Rules

- **Go**: rules_go + Gazelle
- **Rust**: rules_rust + crate_universe
- **C/C++**: Built-in rules (cc_library, cc_binary)
- **Shell**: rules_shell (sh_binary, sh_test)

## Build Commands

**Important**: After modifying code, use the following commands to test that all targets compile and pass tests:

```bash
bazel build //...
bazel test //...
```

```bash
# Go native build
cd go && go build ./cmd/...

# Rust native build
cd rust && cargo build --release

# Bazel build all
bazel build //...

# Bazel build Go CLI
bazel build //go/cmd/...

# Bazel build Rust CLI
bazel build //rust/cmd/...

# Run tests
bazel test //...
```

## External Dependency Management

### Go
Using **rules_go** + **Gazelle**:
1. `go/go.mod` - Defines Go module and dependencies
2. `go/go.sum` - Dependency checksum
3. Run `bazel run //:gazelle` to sync BUILD files

### Rust
Using **crate_universe**:
1. `rust/Cargo.toml` - Workspace definition
2. `rust/Cargo.lock` - Dependency lock
3. Configure `crate.from_cargo()` in `MODULE.bazel`

## CI/CD

GitHub Actions workflow is Bazel-based:
- Code build (`bazel build //...`)
- Unit tests (`bazel test //...`)
- Multi-platform testing (Linux, macOS)
