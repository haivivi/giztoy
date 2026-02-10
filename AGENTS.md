# AGENTS.md - Project Design Document

## Project Overview

giztoy is a Bazel-based multi-language project supporting the following languages:
- Go
- Rust
- C/C++

## Core Design Principles

1. **Language Isolation**: All Go code resides in `go/`, all Rust code resides in `rust/`
2. **Single CLI Entry Point**: `giztoy` is the only Go binary in `go/cmd/`
3. **Examples vs E2E**: `examples/` for SDK usage demos, `e2e/` for integration tests
4. **Dual Build Support**: Supports both native toolchains (go build, cargo) and Bazel builds

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
│   ├── go.mod                # Go module definition (github.com/haivivi/giztoy/go)
│   ├── go.sum                # Go dependency checksum
│   ├── cmd/
│   │   └── giztoy/           # Unified CLI (the only binary)
│   │       ├── commands/     # Subcommands: minimax, doubao, dashscope, cortex, gear, luau
│   │       └── internal/     # Internal packages (config, build)
│   └── pkg/                  # Public libraries
│       ├── audio/            # Audio processing library
│       ├── buffer/           # Buffer utilities
│       ├── chatgear/         # ChatGear device communication
│       ├── cli/              # CLI common utilities
│       ├── dashscope/        # DashScope SDK
│       ├── doubaospeech/     # Doubao Speech SDK
│       ├── embed/            # Embedding interface
│       ├── genx/             # GenX (LLM universal interface)
│       ├── graph/            # Entity-relation graph
│       ├── kv/               # KV store interface
│       ├── luau/             # Luau scripting runtime
│       ├── minimax/          # MiniMax SDK
│       ├── mqtt0/            # MQTT client/broker
│       ├── openai-realtime/  # OpenAI Realtime API
│       ├── recall/           # Search engine
│       ├── storage/          # File storage interface
│       └── vecstore/         # Vector store
│
├── rust/                     # Rust code root directory
│   ├── Cargo.toml            # Rust workspace definition
│   ├── Cargo.lock            # Rust dependency lock
│   ├── cli/                  # CLI common library
│   ├── minimax/              # MiniMax SDK
│   └── cmd/
│       └── minimax/          # MiniMax CLI
│
├── examples/                 # SDK usage examples (for humans)
│   ├── bazel/                # Bazel mobile platform examples
│   │   ├── android/
│   │   ├── ios/
│   │   └── harmonyos/
│   ├── go/                   # Go examples (single go.mod)
│   │   ├── go.mod            # Module: github.com/haivivi/giztoy/examples
│   │   ├── audio/            # Audio processing examples
│   │   ├── dashscope/        # DashScope examples
│   │   ├── doubaospeech/     # Doubao Speech examples
│   │   ├── embed/            # Embedding examples
│   │   ├── minimax/          # MiniMax examples
│   │   ├── mqtt/             # MQTT examples
│   │   └── openai_realtime/  # OpenAI Realtime examples
│   └── rust/                 # Rust examples
│       ├── minimax/
│       ├── dashscope/
│       ├── doubaospeech/
│       ├── audio/
│       ├── genx/
│       └── openai_realtime/
│
├── e2e/                      # Integration tests (for machines)
│   ├── cmd/                  # CLI e2e test runners (Go binaries)
│   │   ├── minimax/          # giztoy minimax tests
│   │   ├── doubaospeech/     # giztoy doubao tests
│   │   └── dashscope/        # giztoy dashscope tests
│   ├── chatgear/             # ChatGear integration tests
│   ├── genx/                 # GenX transformer chain tests
│   ├── doubaospeech/         # Doubao Speech integration tests
│   ├── doubao_minimax/       # Cross-service integration tests
│   └── matchtest/            # Intent matching benchmark
│
└── third_party/              # Third-party C library configurations
    ├── portaudio/
    └── soxr/
```

## Go Directory Description

| Directory | Purpose |
|-----------|---------|
| `go/cmd/giztoy/` | Unified CLI binary with subcommands: minimax, doubao, dashscope, cortex, gear, luau |
| `go/cmd/*/commands/` | Subcommand packages imported by giztoy (no standalone binaries) |
| `go/pkg/` | Public libraries, import path: `github.com/haivivi/giztoy/go/pkg/...` |

## Rust Directory Description

| Directory | Purpose |
|-----------|---------|
| `rust/cmd/` | CLI executables |
| `rust/minimax/` | MiniMax SDK crate |
| `rust/cli/` | CLI common library crate |

## Examples Design

### Go Examples

All Go examples share a single `go.mod` at `examples/go/go.mod`:

```go
// examples/go/go.mod
module github.com/haivivi/giztoy/examples

go 1.25

require github.com/haivivi/giztoy/go v0.0.0

replace github.com/haivivi/giztoy/go => ../../go
```

Benefits:
1. **Single Module**: All Go examples share one module, simplifying dependency management
2. **Local Reference**: Uses `replace` directive to reference the local `go/` module
3. **Native Build**: Can build all examples with `cd examples/go && go build ./...`

### Rust Examples

Rust examples are independent crates at `examples/rust/minimax/`:

```toml
# examples/rust/minimax/Cargo.toml
[package]
name = "minimax-examples"

[dependencies]
giztoy-minimax = { path = "../../../rust/minimax" }

[[bin]]
name = "speech"
path = "src/bin/speech.rs"
```

### E2E Test Runners

CLI e2e test runners are located in `e2e/cmd/`:

```
e2e/cmd/
├── minimax/
│   ├── run.go             # Go test runner
│   ├── BUILD.bazel
│   └── commands/          # YAML command configs
├── doubaospeech/
│   ├── run.go
│   └── BUILD.bazel
└── dashscope/
    ├── run.go
    └── BUILD.bazel
```

Running methods:
```bash
# Bazel execution
bazel run //e2e/cmd/minimax:run -- go 1
bazel run //e2e/cmd/doubaospeech:run -- 1
bazel run //e2e/cmd/dashscope:run -- go all

# Unified CLI
giztoy minimax text chat -f chat.yaml
giztoy doubao tts v2 stream -f tts.yaml
giztoy dashscope omni chat -f omni.yaml
giztoy luau run script.luau
```

## Bazel Rules

- **Go**: rules_go + Gazelle
- **Rust**: rules_rust + crate_universe
- **C/C++**: Built-in rules (cc_library, cc_binary)
- **Shell**: rules_shell (sh_test for luau tests)

## Build Commands

**Important**: After modifying code, use the following commands to test that all targets compile and pass tests:

```bash
bazel build //...
bazel test //...
```

```bash
# Go native build
cd go && go build ./cmd/giztoy/...

# Go examples native build
cd examples/go && go build ./...

# Rust native build
cd rust && cargo build --release

# Bazel build all
bazel build //...

# Bazel build Go CLI
bazel build //go/cmd/giztoy

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
