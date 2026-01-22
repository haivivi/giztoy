//! Simple benchmarks for MQTT protocol encoding/decoding.
//!
//! Run with: bazel run //rust/mqtt0:codec_bench

use std::time::Instant;
use mqtt0::protocol::{codec, v4};

const ITERATIONS: u32 = 100_000;

fn bench<F: FnMut()>(name: &str, mut f: F) {
    // Warmup
    for _ in 0..1000 {
        f();
    }
    
    let start = Instant::now();
    for _ in 0..ITERATIONS {
        f();
    }
    let elapsed = start.elapsed();
    
    let per_op = elapsed / ITERATIONS;
    let ops_per_sec = if per_op.as_nanos() > 0 {
        1_000_000_000 / per_op.as_nanos()
    } else {
        0
    };
    
    println!(
        "{:40} {:>10.2?} per op, {:>12} ops/sec",
        name,
        per_op,
        format_number(ops_per_sec as u64)
    );
}

fn format_number(n: u64) -> String {
    if n >= 1_000_000 {
        format!("{:.2}M", n as f64 / 1_000_000.0)
    } else if n >= 1_000 {
        format!("{:.2}K", n as f64 / 1_000.0)
    } else {
        format!("{}", n)
    }
}

fn bench_variable_int() {
    println!("\n=== Variable Length Integer ===");
    
    let mut buf = [0u8; 4];
    
    bench("encode 127 (1 byte)", || {
        let _ = codec::write_variable_int(&mut buf, 127);
    });
    
    bench("encode 16383 (2 bytes)", || {
        let _ = codec::write_variable_int(&mut buf, 16383);
    });
    
    bench("encode 2097151 (3 bytes)", || {
        let _ = codec::write_variable_int(&mut buf, 2097151);
    });
    
    bench("encode 268435455 (4 bytes)", || {
        let _ = codec::write_variable_int(&mut buf, 268435455);
    });
    
    // Decode benchmarks
    let encoded_1 = [0x7F];
    let encoded_2 = [0xFF, 0x7F];
    let encoded_3 = [0xFF, 0xFF, 0x7F];
    let encoded_4 = [0xFF, 0xFF, 0xFF, 0x7F];
    
    bench("decode 127 (1 byte)", || {
        let _ = codec::read_variable_int(&encoded_1);
    });
    
    bench("decode 16383 (2 bytes)", || {
        let _ = codec::read_variable_int(&encoded_2);
    });
    
    bench("decode 2097151 (3 bytes)", || {
        let _ = codec::read_variable_int(&encoded_3);
    });
    
    bench("decode 268435455 (4 bytes)", || {
        let _ = codec::read_variable_int(&encoded_4);
    });
}

fn bench_publish_packet() {
    println!("\n=== PUBLISH Packet ===");
    
    // Small payload (100 bytes)
    let small_payload = vec![0u8; 100];
    let small_packet = v4::Packet::Publish(v4::Publish {
        topic: "test/topic".to_string(),
        payload: bytes::Bytes::from(small_payload),
        qos: mqtt0::QoS::AtMostOnce,
        retain: false,
        dup: false,
        pkid: 0,
    });
    
    let mut buf_small = vec![0u8; 200];
    bench("encode 100B payload", || {
        let _ = small_packet.write(&mut buf_small);
    });
    
    let len = small_packet.write(&mut buf_small).unwrap();
    let encoded_small = buf_small[..len].to_vec();
    
    bench("decode 100B payload", || {
        let _ = v4::Packet::read(&encoded_small, 1024 * 1024);
    });
    
    // Medium payload (1KB)
    let medium_payload = vec![0u8; 1024];
    let medium_packet = v4::Packet::Publish(v4::Publish {
        topic: "test/topic".to_string(),
        payload: bytes::Bytes::from(medium_payload),
        qos: mqtt0::QoS::AtMostOnce,
        retain: false,
        dup: false,
        pkid: 0,
    });
    
    let mut buf_medium = vec![0u8; 2000];
    bench("encode 1KB payload", || {
        let _ = medium_packet.write(&mut buf_medium);
    });
    
    let len = medium_packet.write(&mut buf_medium).unwrap();
    let encoded_medium = buf_medium[..len].to_vec();
    
    bench("decode 1KB payload", || {
        let _ = v4::Packet::read(&encoded_medium, 1024 * 1024);
    });
    
    // Large payload (10KB)
    let large_payload = vec![0u8; 10240];
    let large_packet = v4::Packet::Publish(v4::Publish {
        topic: "test/topic".to_string(),
        payload: bytes::Bytes::from(large_payload),
        qos: mqtt0::QoS::AtMostOnce,
        retain: false,
        dup: false,
        pkid: 0,
    });
    
    let mut buf_large = vec![0u8; 12000];
    bench("encode 10KB payload", || {
        let _ = large_packet.write(&mut buf_large);
    });
    
    let len = large_packet.write(&mut buf_large).unwrap();
    let encoded_large = buf_large[..len].to_vec();
    
    bench("decode 10KB payload", || {
        let _ = v4::Packet::read(&encoded_large, 1024 * 1024);
    });
}

fn bench_connect_packet() {
    println!("\n=== CONNECT Packet ===");
    
    let connect = v4::Connect {
        client_id: "benchmark-client-12345".to_string(),
        keep_alive: 60,
        clean_session: true,
        username: Some("username".to_string()),
        password: Some(b"password".to_vec()),
        will: None,
    };
    
    let mut buf = vec![0u8; 200];
    bench("encode CONNECT", || {
        let _ = connect.write(&mut buf);
    });
    
    let len = connect.write(&mut buf).unwrap();
    // Skip fixed header for decode (header is 2 bytes for this packet)
    let payload = buf[2..len].to_vec();
    
    bench("decode CONNECT", || {
        let _ = v4::Connect::read(&payload);
    });
}

fn bench_subscribe_packet() {
    println!("\n=== SUBSCRIBE Packet ===");
    
    let topics = ["topic/1", "topic/2", "topic/3", "topic/+/wildcard", "topic/#"];
    let packet = v4::create_subscribe(1, &topics);
    
    let mut buf = vec![0u8; 200];
    bench("encode SUBSCRIBE (5 topics)", || {
        let _ = packet.write(&mut buf);
    });
    
    let len = packet.write(&mut buf).unwrap();
    let encoded = buf[..len].to_vec();
    
    bench("decode SUBSCRIBE (5 topics)", || {
        let _ = v4::Packet::read(&encoded, 1024 * 1024);
    });
}

fn bench_throughput() {
    println!("\n=== Throughput Test ===");
    
    // Simulate encoding/decoding many small messages
    let packet = v4::Packet::Publish(v4::Publish {
        topic: "sensor/temp".to_string(),
        payload: bytes::Bytes::from_static(b"25.5"),
        qos: mqtt0::QoS::AtMostOnce,
        retain: false,
        dup: false,
        pkid: 0,
    });
    
    let mut buf = vec![0u8; 100];
    let len = packet.write(&mut buf).unwrap();
    let encoded = buf[..len].to_vec();
    
    let iterations = 1_000_000u64;
    
    // Encode throughput
    let start = Instant::now();
    for _ in 0..iterations {
        let _ = packet.write(&mut buf);
    }
    let elapsed = start.elapsed();
    let msg_per_sec = iterations * 1_000_000_000 / elapsed.as_nanos() as u64;
    println!(
        "{:40} {:>12} msg/sec",
        "encode small message throughput",
        format_number(msg_per_sec)
    );
    
    // Decode throughput
    let start = Instant::now();
    for _ in 0..iterations {
        let _ = v4::Packet::read(&encoded, 1024 * 1024);
    }
    let elapsed = start.elapsed();
    let msg_per_sec = iterations * 1_000_000_000 / elapsed.as_nanos() as u64;
    println!(
        "{:40} {:>12} msg/sec",
        "decode small message throughput",
        format_number(msg_per_sec)
    );
}

fn main() {
    println!("mqtt0 Protocol Benchmark");
    println!("========================");
    println!("Iterations per benchmark: {}", ITERATIONS);
    
    bench_variable_int();
    bench_publish_packet();
    bench_connect_packet();
    bench_subscribe_packet();
    bench_throughput();
    
    println!("\nBenchmark complete!");
}
