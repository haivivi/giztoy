# Backlog: Rename examples/ to e2e/, Unify testdata Pattern

## Background

The `cl/go/doubaospeech-cli` branch established a new pattern for organizing
end-to-end tests and shared test data. This backlog tracks the work to apply
this pattern across the entire project.

## What Changed (doubaospeech-cli branch)

```
# Before
examples/
├── cmd/doubaospeech/
│   ├── run.sh              # Platform-dependent shell script
│   ├── commands/*.yaml     # Request configs (mixed with runner)
│   └── testdata/           # Test audio (buried in examples)
├── go/                     # Go SDK examples
└── rust/                   # Rust SDK examples

# After
testdata/
├── doubaospeech/
│   ├── commands/*.yaml     # Request configs (shared Go + Rust)
│   ├── templates/*.yaml
│   └── test_speech.mp3     # Test audio (git-lfs)
e2e/
├── cmd/doubaospeech/
│   ├── run.go              # Cross-platform Go executor
│   └── BUILD.bazel         # go_binary, tags = ["manual"]
├── go/                     # Go SDK examples
└── rust/                   # Rust SDK examples
```

## Design Decisions

### 1. testdata/ is top-level, language-agnostic

Test data (YAML request configs, audio files, model configs) lives in
`testdata/` at the project root. Both Go and Rust CLI runners reference
the same data. No duplication.

```
testdata/
├── doubaospeech/commands/   # CLI request YAML configs
├── models/                  # LLM model configs (already existed)
├── matchtest/               # Match test rules (already existed)
└── luau/                    # Luau test scripts (already existed)
```

### 2. e2e/cmd/ runners are Go binaries, not shell scripts

Per workspace rule: "不要在 Bazel 中使用 sh_binary / sh_test，用 Go 写跨平台的
executor（go_binary），避免平台依赖"

Each CLI tool gets a Go-based test runner:

```
e2e/cmd/<tool>/
├── run.go          # package main, invokes the CLI binary
└── BUILD.bazel     # go_binary with tags = ["manual"]
```

The runner:
- Locates the CLI binary via `BUILD_WORKSPACE_DIRECTORY` (bazel run) or PATH
- Reads YAML configs from `testdata/<tool>/commands/`
- Sets up context from env vars (`<TOOL>_APP_ID`, `<TOOL>_API_KEY`)
- Runs tests by level/tag, reports pass/fail with colored output
- Writes output files to `e2e/cmd/<tool>/output/` (gitignored)

### 3. BUILD.bazel pattern

```starlark
go_binary(
    name = "run",
    embed = [":run_lib"],
    data = [
        "//go/cmd/<tool>",           # The CLI binary under test
        "//testdata/<tool>:<tool>",   # Shared test data
    ],
    tags = ["manual"],  # Requires API credentials, not for CI //...
)
```

### 4. testdata BUILD.bazel pattern

```starlark
filegroup(
    name = "<tool>",
    srcs = glob(
        ["commands/*.yaml", "templates/*.yaml", "*.mp3", "*.pcm"],
        allow_empty = True,
    ),
    visibility = ["//visibility:public"],
)
```

### 5. Binary audio/media in testdata use git-lfs

```gitattributes
testdata/doubaospeech/*.mp3 filter=lfs diff=lfs merge=lfs -text
testdata/doubaospeech/*.pcm filter=lfs diff=lfs merge=lfs -text
```

## TODO: Apply to Other CLI Tools

### minimax (priority: high)

Currently: `examples/cmd/minimax/run.sh` (shell script)

Target:
```
testdata/minimax/commands/*.yaml   # Move from examples/cmd/minimax/commands/
e2e/cmd/minimax/run.go             # New Go executor
e2e/cmd/minimax/BUILD.bazel        # go_binary, tags = ["manual"]
```

### dashscope (priority: medium)

Currently: `examples/cmd/dashscope/run.sh` (shell script)

Target:
```
testdata/dashscope/commands/*.yaml
e2e/cmd/dashscope/run.go
e2e/cmd/dashscope/BUILD.bazel
```

### Bulk rename examples/ to e2e/

The rename was already done on the `cl/go/doubaospeech-cli` branch.
When merging to main, the `examples/` -> `e2e/` rename needs to be
applied globally. Key files to update:

- `e2e/go/go.mod`: module path `github.com/haivivi/giztoy/e2e`
- `e2e/go/BUILD.bazel`: gazelle prefix
- All `e2e/**/BUILD.bazel`: importpath references
- `AGENTS.md`: documentation references
- `.github/workflows/`: CI references
- `README.md`: usage examples

## Acceptance Criteria

- [ ] No `sh_binary` / `sh_test` / `run.sh` in `e2e/cmd/`
- [ ] All YAML request configs in `testdata/<tool>/commands/`
- [ ] All test audio/media in `testdata/<tool>/` with git-lfs
- [ ] All e2e runners are `go_binary` with `tags = ["manual"]`
- [ ] `bazel build //...` does not trigger e2e targets
- [ ] `bazel run //e2e/cmd/<tool>:run -- quick` works for each tool
