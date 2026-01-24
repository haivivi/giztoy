# ESP32 Rust (esp-rs) 固件开发指南

本文档记录了将现有 C/ESP-IDF 固件迁移到 Rust (esp-rs) 的可行性分析和技术方案。

## 目录

- [背景](#背景)
- [技术栈对比](#技术栈对比)
- [各模块迁移分析](#各模块迁移分析)
- [依赖管理方案](#依赖管理方案)
- [AEC 回声消除方案](#aec-回声消除方案)
- [开发和调试工具](#开发和调试工具)
- [工作量评估](#工作量评估)

---

## 背景

现有固件基于 C + ESP-IDF + ESP-ADF 开发，主要痛点：
- ESP-ADF 代码质量不佳，维护困难
- C 语言缺乏现代语言特性，开发效率低
- 内存安全问题难以排查

目标：使用 Rust (esp-rs) 重写固件，保持功能不变，提升代码质量和开发效率。

---

## 技术栈对比

| 组件 | 现有方案 (C) | Rust 方案 |
|------|-------------|-----------|
| 框架 | ESP-IDF | esp-idf-hal + esp-idf-svc |
| 音频框架 | ESP-ADF | 自行实现（移除 ESP-ADF） |
| GUI | LVGL (C) | lvgl-rs (binding) |
| BLE | NimBLE (C) | esp32-nimble (Rust) |
| Opus 编解码 | libopus (C) | opus crate (binding) |
| AEC | esp_afe (闭源) | esp_afe binding / Speex AEC |
| 内存管理 | heap_caps_malloc | esp-alloc + PSRAM 支持 |
| 构建系统 | CMake (idf.py) | Cargo + build.rs |

---

## 各模块迁移分析

### 1. FreeRTOS 任务管理 ✅

**现有代码** (`hvv_task.c`):
```c
TaskHandle_t hvv_freertos_task_create(const char *name, hvv_task_func_t func,
                                      void *arg, hvv_task_options_t options) {
    stack = (StackType_t *)heap_caps_malloc(
        stack_size, MALLOC_CAP_SPIRAM | MALLOC_CAP_8BIT);  // PSRAM
    // ...
}
```

**Rust 方案**:
```rust
use esp_idf_hal::task::*;
use esp_idf_sys::*;

fn create_task_psram<F>(name: &str, stack_size: usize, f: F)
where
    F: FnOnce() + Send + 'static,
{
    // esp-idf-hal 支持 PSRAM 任务栈
    let config = TaskConfig {
        stack_size,
        priority: 5,
        // stack_in_psram: true,  // 需要检查 API
        ..Default::default()
    };
    std::thread::Builder::new()
        .stack_size(stack_size)
        .spawn(f)
        .unwrap();
}
```

**可行性**: ✅ 完全可行，esp-idf-hal 支持 PSRAM 分配

---

### 2. BLE 通信 ✅

**现有代码**: 基于 NimBLE 的自定义 BLE Server，包含复杂的分块传输协议。

**Rust 方案**: 使用 `esp32-nimble` crate

```rust
use esp32_nimble::{BLEDevice, BLEServer};

fn init_ble() -> anyhow::Result<()> {
    let device = BLEDevice::take();
    let server = device.get_server();
    
    let service = server.create_service(uuid128!("12345678-..."));
    let characteristic = service.create_characteristic(
        uuid128!("87654321-..."),
        NimbleProperties::READ | NimbleProperties::WRITE | NimbleProperties::NOTIFY,
    );
    
    characteristic.on_write(|args| {
        // 处理写入
    });
    
    Ok(())
}
```

**可行性**: ✅ esp32-nimble 功能完整，可以实现现有的分块传输协议

---

### 3. LVGL GUI ✅

**现有代码** (`lvgl_disp.c`): 约 200 行显示驱动代码

```c
static void disp_flush_cb(lv_display_t *disp, const lv_area_t *area,
                          uint8_t *px_map) {
    lv_draw_sw_rgb565_swap(px_map, width * height);
    esp_lcd_panel_draw_bitmap(panel, x1, y1, x2 + 1, y2 + 1, px_map);
}
```

**Rust 方案**: 使用 `lvgl-rs`

```rust
use lvgl::{Display, DrawBuffer};

extern "C" fn disp_flush_cb(
    disp: *mut lv_display_t,
    area: *const lv_area_t,
    px_map: *mut u8,
) {
    // RGB565 swap
    // esp_lcd_panel_draw_bitmap (通过 FFI)
    unsafe { lv_display_flush_ready(disp) };
}

fn init_display() -> anyhow::Result<()> {
    let buffer = DrawBuffer::<{ 240 * 240 }>::new();
    let display = Display::register(buffer, 240, 240, disp_flush_cb)?;
    Ok(())
}
```

**可行性**: ✅ lvgl-rs 是官方 binding，显示驱动只需翻译约 50 行代码

---

### 4. 音频处理 ⚠️

这是最复杂的部分，需要移除 ESP-ADF 并自行实现。

#### 4.1 I2S 音频输入输出 ✅

```rust
use esp_idf_hal::i2s::*;

fn audio_input_task() -> anyhow::Result<()> {
    let i2s = I2sDriver::new_std_rx(
        peripherals.i2s0,
        &StdRxConfig::new(
            16000,   // sample rate
            I2sSlotMode::Stereo,
            I2sDataBitWidth::Bits16,
        ),
        pins,
    )?;
    
    let mut buffer = [0i16; 320];  // 20ms @ 16kHz
    loop {
        i2s.read(&mut buffer, BLOCK)?;
        // 处理音频...
    }
}
```

#### 4.2 Opus 编解码 ✅

```rust
use opus::{Encoder, Decoder, Application};

let mut encoder = Encoder::new(16000, opus::Channels::Mono, Application::Voip)?;
let mut decoder = Decoder::new(16000, opus::Channels::Mono)?;

// 编码
let encoded = encoder.encode(&pcm_data, &mut output)?;

// 解码
let decoded = decoder.decode(&opus_data, &mut pcm_output, false)?;
```

#### 4.3 重采样 ⚠️

**不推荐 soxr** (性能和内存要求太高)

**推荐方案**:
- ESP-ADF 内置 resampler (通过 FFI)
- Speex resampler (轻量级)

```rust
// Speex resampler binding
extern "C" {
    fn speex_resampler_init(
        nb_channels: u32,
        in_rate: u32,
        out_rate: u32,
        quality: i32,
        err: *mut i32,
    ) -> *mut SpeexResamplerState;
}
```

---

### 5. AEC 回声消除 ⚠️

#### 硬件情况

ES8311 codec **不是硬件 AEC**！它只是把 MIC 和 Speaker 参考信号同步输出到 I2S：

```
ES8311 I2S 输出:
┌─────────────────────────────────┐
│  Left Channel  │ Right Channel │
│  (MIC 原始)    │ (Speaker 参考) │
└─────────────────────────────────┘
          ↓
    还需要软件 AEC！
```

#### 方案选择

| 方案 | 优点 | 缺点 |
|------|------|------|
| **esp_afe binding** | 效果最好，官方维护 | 闭源，需要写 bindgen |
| **Speex AEC** | 开源，轻量 | 效果一般 |
| **自己实现 NLMS** | 完全可控 | 效果差，工作量大 |

**推荐**: esp_afe binding

```rust
// esp_afe FFI binding (通过 bindgen 生成)
use esp_afe_sys::*;

fn init_afe() -> *mut esp_afe_sr_iface_t {
    unsafe {
        let config = afe_config_t {
            // ...
        };
        esp_afe_sr_create(&config)
    }
}

fn process_audio(afe: *mut esp_afe_sr_iface_t, mic: &[i16], ref_: &[i16]) -> Vec<i16> {
    unsafe {
        // 调用 esp_afe 处理
    }
}
```

---

## 依赖管理方案

### Bazel 管理 C 依赖

现有项目已使用 Bazel 管理 `opus`, `ogg` 等 C 依赖。可以继续使用：

```python
# WORKSPACE
http_archive(
    name = "opus",
    urls = ["https://github.com/xiph/opus/archive/v1.5.2.tar.gz"],
    strip_prefix = "opus-1.5.2",
)
```

### ESP-IDF CMake 引用 Bazel 输出

ESP-IDF 的 CMakeLists.txt 可以直接引用 Bazel 下载的源码：

```cmake
# components/opus/CMakeLists.txt

# 获取 Bazel external 目录
execute_process(
    COMMAND bazel info output_base
    OUTPUT_VARIABLE BAZEL_OUTPUT_BASE
    OUTPUT_STRIP_TRAILING_WHITESPACE
    WORKING_DIRECTORY ${CMAKE_SOURCE_DIR}/../../..
)

# 指向 Bazel 下载的 opus 源码
set(OPUS_DIR "${BAZEL_OUTPUT_BASE}/external/opus")

# 使用这些源码编译
file(GLOB_RECURSE OPUS_SRCS "${OPUS_DIR}/src/*.c")
idf_component_register(
    SRCS ${OPUS_SRCS}
    INCLUDE_DIRS "${OPUS_DIR}/include"
)
```

### Cargo 链接 Bazel 构建产物

```rust
// build.rs
fn main() {
    let bazel_output = Command::new("bazel")
        .args(["info", "output_base"])
        .output()
        .expect("bazel not found");
    
    let external_dir = format!(
        "{}/external",
        String::from_utf8_lossy(&bazel_output.stdout).trim()
    );
    
    println!("cargo:rustc-link-search={}/opus/lib", external_dir);
    println!("cargo:rustc-link-lib=static=opus");
}
```

---

## 开发和调试工具

### 构建和烧录

```bash
# 安装 esp-rs 工具链
cargo install espup
espup install

# 构建
cargo build --release

# 烧录 (不需要 idf.py)
cargo espflash flash --release --monitor
```

### 模拟器

#### QEMU (本地)

```bash
# 安装 QEMU ESP32 支持
# (目前官方支持有限，主要用于简单测试)
qemu-system-xtensa -machine esp32 -nographic -drive file=flash.bin,format=raw
```

#### Wokwi (在线)

- 网址: https://wokwi.com/
- 支持 ESP32/ESP32-S3
- 可以模拟 LED、按钮、显示屏等外设
- 支持 Rust 项目

```json
// wokwi.toml
[wokwi]
version = 1
firmware = "target/xtensa-esp32s3-espidf/release/my-project"
elf = "target/xtensa-esp32s3-espidf/release/my-project"

[[parts]]
id = "esp32s3"
x = 0
y = 0
```

**限制**: 
- 不支持真实的 I2S 音频
- BLE 模拟有限
- 主要用于 GPIO/显示逻辑测试

---

## 工作量评估

| 模块 | 工作量 | 风险 |
|------|--------|------|
| 项目结构搭建 | 1 周 | 低 |
| FreeRTOS 任务封装 | 1 周 | 低 |
| BLE 通信 | 2 周 | 中 |
| LVGL 显示驱动 | 1 周 | 低 |
| UI 应用层 | 3 周 | 中 |
| I2S 音频 | 1 周 | 低 |
| Opus 编解码 | 1 周 | 低 |
| AEC (esp_afe binding) | 2 周 | **高** |
| MQTT 通信 | 1 周 | 低 |
| NVS 存储 | 0.5 周 | 低 |
| 测试和调试 | 2 周 | 中 |
| **总计** | **~15 周** | - |

---

## 参考资源

- [esp-rs Book](https://esp-rs.github.io/book/)
- [esp-idf-hal](https://github.com/esp-rs/esp-idf-hal)
- [esp-idf-svc](https://github.com/esp-rs/esp-idf-svc)
- [esp32-nimble](https://github.com/taks/esp32-nimble)
- [lvgl-rs](https://github.com/lvgl/lv_binding_rust)
- [Wokwi Simulator](https://wokwi.com/)

---

## 下一步

1. ✅ 创建 giztoy Rust 项目骨架
2. ✅ 整合 opus/ogg 组件到 `esp/components/`
3. ✅ 创建 ESP-RS hello world 示例 (`esp/rust/hello/`)
4. ✅ 创建 Opus codec Rust 示例 (`esp/rust/opus-example/`)
5. ✅ **Hello World 在 ESP32-S3 上运行成功** (2026-01-22)
   - 堆内存: 8.7MB (PSRAM 已启用)
   - ESP-IDF v5.3 + Rust 1.90.0
6. ⬜ 实现 LED 闪烁 Demo
7. ⬜ 移植 LVGL 显示驱动
8. ⬜ 移植 BLE 通信
9. ⬜ 移植音频输入输出 (I2S)
10. ⬜ 集成 esp_afe AEC
