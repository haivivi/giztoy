# AGENTS.md - Project Design Document

## Project Overview

giztoy is a Bazel-based multi-language project supporting the following languages:
- Go
- Rust
- C/C++

## Core Design Principles

1. **Language Isolation**: All Go code resides in `go/`, all Rust code resides in `rust/`
2. **Independent Examples**: Examples are organized by language under `examples/`, each with its own module definition
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
├── examples/                 # Example code
│   ├── cmd/                  # CLI test scripts (shell)
│   │   ├── minimax/          # MiniMax CLI test scripts
│   │   │   ├── run.sh
│   │   │   └── commands/     # YAML command configs
│   │   └── doubaospeech/     # Doubao Speech CLI test scripts
│   │       ├── run.sh
│   │       └── commands/     # YAML command configs
│   ├── go/                   # Go examples (single go.mod)
│   │   ├── go.mod            # Module: github.com/haivivi/giztoy/examples
│   │   ├── audio/            # Audio processing examples
│   │   ├── dashscope/        # DashScope examples
│   │   ├── doubaospeech/     # Doubao Speech examples
│   │   ├── genx/             # GenX examples
│   │   └── minimax/          # MiniMax examples
│   └── rust/                 # Rust examples
│       └── minimax/          # MiniMax Rust examples
│           ├── Cargo.toml
│           └── src/bin/      # Binary examples
│
└── third_party/              # Third-party C library configurations
    ├── portaudio/
    └── soxr/
```

## Go Directory Description

| Directory | Purpose |
|-----------|---------|
| `go/cmd/` | CLI executables, each subdirectory is an independent command-line tool |
| `go/pkg/` | Public libraries, import path: `github.com/haivivi/giztoy/pkg/...` |

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

require github.com/haivivi/giztoy v0.0.0

replace github.com/haivivi/giztoy => ../../go
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

### CLI Test Scripts

CLI test scripts are located in `examples/cmd/`:

```
examples/cmd/
├── minimax/
│   ├── run.sh              # Test runner
│   ├── BUILD.bazel
│   └── commands/           # YAML command configs
│       ├── chat.yaml
│       ├── speech.yaml
│       └── ...
└── doubaospeech/
    ├── run.sh
    ├── BUILD.bazel
    └── commands/
        ├── tts.yaml
        └── ...
```

Running methods:
```bash
# Direct execution
./examples/cmd/minimax/run.sh go 1
./examples/cmd/doubaospeech/run.sh tts

# Bazel execution
bazel run //examples/cmd/minimax:run -- go 1
bazel run //examples/cmd/doubaospeech:run -- tts
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

# Go examples native build
cd examples/go && go build ./...

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
