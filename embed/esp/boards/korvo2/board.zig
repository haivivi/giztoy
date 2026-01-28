//! ESP32-S3-Korvo-2 v3.1 audio development board configuration
//!
//! A feature-rich audio development board with:
//! - Dual microphones (digital PDM)
//! - Speaker output via I2S codec (ES8311)
//! - Audio PA (NS4150)
//! - Function buttons
//! - RGB LEDs (WS2812)
//! - LCD display interface
//! - Camera interface
//! - SD card slot
//! - PSRAM (8MB octal)

const std = @import("std");

/// Board metadata
pub const name = "korvo2";
pub const chip = "esp32s3";
pub const description = "ESP32-S3-Korvo-2 v3.1 audio development board";

/// GPIO pin assignments
pub const gpio = struct {
    // I2S Audio (ES8311 codec)
    pub const i2s_mclk = 16;
    pub const i2s_bclk = 9;
    pub const i2s_ws = 45;
    pub const i2s_dout = 8;  // Speaker data out
    pub const i2s_din = 10; // Mic data in

    // Audio codec control (ES8311)
    pub const codec_i2c_sda = 17;
    pub const codec_i2c_scl = 18;
    pub const codec_pa_enable = 48; // NS4150 PA enable

    // PDM Microphones (dual digital mics)
    pub const pdm_clk = 42;
    pub const pdm_din = 41;

    // Function buttons
    pub const btn_boot = 0;
    pub const btn_rec = 46;   // Record button
    pub const btn_mode = 1;   // Mode button
    pub const btn_play = 21;  // Play/Pause button
    pub const btn_set = 47;   // Settings button
    pub const btn_vol_up = 40;
    pub const btn_vol_down = 39;

    // RGB LEDs (WS2812, directly connected LED strip)
    pub const led_rgb = 19;
    pub const led_count = 12; // Number of LEDs on the ring

    // LCD display (directly accessible, directly connect if needed)
    pub const lcd_cs = 5;
    pub const lcd_dc = 4;
    pub const lcd_rst = 6;
    pub const lcd_blk = 7;
    pub const lcd_sclk = 15;
    pub const lcd_mosi = 11;

    // SD card (directly accessible)
    pub const sd_cmd = 14;
    pub const sd_clk = 12;
    pub const sd_d0 = 13;

    // Camera interface (directly accessible, directly configure)
    pub const cam_pwdn = -1; // Not connected
    pub const cam_reset = -1; // Not connected
    pub const cam_xclk = 2;
    pub const cam_siod = 17; // Shared with codec
    pub const cam_sioc = 18; // Shared with codec
    pub const cam_d0 = 37;
    pub const cam_d1 = 36;
    pub const cam_d2 = 35;
    pub const cam_d3 = 34;
    pub const cam_d4 = 33;
    pub const cam_d5 = 32;
    pub const cam_d6 = 31;
    pub const cam_d7 = 30;
    pub const cam_vsync = 3;
    pub const cam_href = 38;
    pub const cam_pclk = 29;
};

/// Power management
pub const power = struct {
    pub fn init() void {
        // Enable audio PA
        enablePA(false); // Start with PA disabled
    }

    pub fn enablePA(enable: bool) void {
        _ = enable;
        // TODO: Set GPIO48 to enable/disable NS4150 PA
    }
};

/// Audio codec (ES8311) controller
pub const codec = struct {
    const ES8311_ADDR: u8 = 0x18;

    pub fn init() void {
        // Initialize I2C
        // Configure ES8311 registers
    }

    pub fn setVolume(volume: u8) void {
        _ = volume;
        // TODO: Set DAC volume register
    }

    pub fn setMicGain(gain: u8) void {
        _ = gain;
        // TODO: Set ADC gain register
    }
};

/// Speaker output
pub const speaker = struct {
    pub fn init() void {
        codec.init();
        power.init();
    }

    pub fn enable() void {
        power.enablePA(true);
    }

    pub fn disable() void {
        power.enablePA(false);
    }

    pub fn setVolume(volume: u8) void {
        codec.setVolume(volume);
    }
};

/// Microphone input (dual PDM mics)
pub const mic = struct {
    pub fn init() void {
        // Configure PDM interface
    }

    pub fn setGain(gain: u8) void {
        codec.setMicGain(gain);
    }
};

/// LED ring controller (WS2812 x 12)
pub const led = struct {
    pub fn init() void {
        // Initialize RMT peripheral for WS2812
    }

    pub fn setPixel(index: usize, r: u8, g: u8, b: u8) void {
        _ = index;
        _ = r;
        _ = g;
        _ = b;
        // TODO: Set LED buffer
    }

    pub fn fill(r: u8, g: u8, b: u8) void {
        for (0..gpio.led_count) |i| {
            setPixel(i, r, g, b);
        }
    }

    pub fn show() void {
        // TODO: Send data via RMT
    }
};

/// Button controller
pub const button = struct {
    pub const Button = enum {
        boot,
        rec,
        mode,
        play,
        set,
        vol_up,
        vol_down,
    };

    pub fn init() void {
        // Configure all button GPIOs as input with pull-up
    }

    pub fn isPressed(btn: Button) bool {
        _ = btn;
        // TODO: Read GPIO state
        return false;
    }
};

/// Board initialization
pub fn init() void {
    power.init();
    codec.init();
    speaker.init();
    mic.init();
    led.init();
    button.init();
}

/// High-level board interface
pub const Board = struct {
    pub const Speaker = speaker;
    pub const Mic = mic;
    pub const Led = led;
    pub const Button = button;
    pub const Power = power;

    pub fn setup() void {
        init();
    }
};
