# AGENTS.md - 项目设计文档

## 项目概述

giztoy 是一个基于 Bazel 的多语言项目，支持以下语言：
- Go
- C
- Zig
- Rust

## 目录结构设计

```
giztoy/
├── MODULE.bazel              # Bazel 模块定义（bzlmod，推荐 Bazel 7+）
├── WORKSPACE.bazel           # 传统 workspace 文件（可作为后备）
├── BUILD.bazel               # 根 BUILD 文件
├── .bazelrc                  # Bazel 配置
├── .bazelversion             # Bazel 版本锁定
├── .github/
│   └── workflows/
│       └── ci.yaml           # GitHub Actions CI
├── README.md
│
├── go/                       # Go 代码根目录
│   ├── BUILD.bazel
│   ├── go.mod                # Go 模块定义
│   ├── go.sum                # Go 依赖校验
│   ├── cmd/                  # 可执行程序入口点
│   │   └── example/
│   │       ├── BUILD.bazel
│   │       └── main.go
│   ├── pkg/                  # 公共库（可被外部引用）
│   │   └── utils/
│   │       ├── BUILD.bazel
│   │       ├── utils.go
│   │       └── utils_test.go
│   └── internal/             # 内部私有库
│       └── helper/
│           ├── BUILD.bazel
│           └── helper.go
│
├── c/                        # C 代码目录
│   ├── BUILD.bazel
│   └── src/
│
├── zig/                      # Zig 代码目录
│   ├── BUILD.bazel
│   └── src/
│
└── rust/                     # Rust 代码目录
    ├── BUILD.bazel
    └── src/
```

## Go 目录说明

| 目录 | 用途 |
|------|------|
| `go/cmd/` | 放置 `main` 包，每个子目录是一个独立的可执行程序 |
| `go/pkg/` | 公共库代码，可被项目内外部引用 |
| `go/internal/` | 内部库，Go 语言会强制限制只能被同级或下级目录引用 |

## Bazel 规则

- **Go**: rules_go + Gazelle
- **C/C++**: 内置规则 (cc_library, cc_binary)
- **Zig**: rules_zig
- **Rust**: rules_rust

## 外部 Go 包管理

使用 **rules_go** + **Gazelle** 来管理外部依赖：

1. `go.mod` - 定义 Go 模块和依赖
2. `go.sum` - 依赖校验
3. 运行 `bazel run //:gazelle -- update-repos -from_file=go/go.mod` 来同步依赖到 Bazel

## CI/CD

GitHub Actions workflow 基于 Bazel 进行：
- 代码构建
- 单元测试
- 多平台测试（Linux, macOS）
