# Buffer Package - Rust Implementation

Crate: `giztoy-buffer`

📚 [Rust Documentation](https://docs.rs/giztoy-buffer)

## Types

### Buffer<T>

Growable buffer using `VecDeque<T>` for O(1) front operations.

```rust
pub struct Buffer<T> {
    inner: Arc<BufferInner<T>>,
}

struct BufferInner<T> {
    state: Mutex<BufferState<T>>,
    write_notify: Condvar,
}

struct BufferState<T> {
    buf: VecDeque<T>,
    close_write: bool,
    close_err: Option<Arc<dyn Error + Send + Sync>>,
}
```

**Key Methods:**

| Method | Signature | Description |
|--------|-----------|-------------|
| `new` | `fn new() -> Self` | Create empty buffer |
| `with_capacity` | `fn with_capacity(capacity: usize) -> Self` | Create with capacity hint |
| `write` | `fn write(&self, data: &[T]) -> Result<usize, BufferError>` | Append elements |
| `read` | `fn read(&self, buf: &mut [T]) -> Result<usize, BufferError>` | Read elements (blocks) |
| `add` | `fn add(&self, item: T) -> Result<(), BufferError>` | Add single element |
| `next` | `fn next(&self) -> Result<T, Done>` | Pop from front (FIFO) |
| `to_vec` | `fn to_vec(&self) -> Vec<T>` | Copy to Vec |

### BlockBuffer<T>

Fixed-size circular buffer with blocking semantics.

```rust
pub struct BlockBuffer<T> {
    inner: Arc<BlockBufferInner<T>>,
}

struct BlockBufferInner<T> {
    state: Mutex<BlockBufferState<T>>,
    not_full: Condvar,
    not_empty: Condvar,
}

struct BlockBufferState<T> {
    buf: Vec<Option<T>>,
    head: usize,
    tail: usize,
    count: usize,
    close_write: bool,
    close_err: Option<Arc<dyn Error + Send + Sync>>,
}
```

**Key Methods:**

| Method | Signature | Description |
|--------|-----------|-------------|
| `new` | `fn new(capacity: usize) -> Self` | Create with capacity |
| `from_vec` | `fn from_vec(data: Vec<T>) -> Self` | Create from Vec (full) |
| `write` | `fn write(&self, data: &[T]) -> Result<usize, BufferError>` | Write (blocks when full) |
| `read` | `fn read(&self, buf: &mut [T]) -> Result<usize, BufferError>` | Read (blocks when empty) |
| `capacity` | `fn capacity(&self) -> usize` | Get capacity |
| `is_full` | `fn is_full(&self) -> bool` | Check if full |

### RingBuffer<T>

Fixed-size circular buffer with overwrite semantics.

```rust
pub struct RingBuffer<T> {
    inner: Arc<RingBufferInner<T>>,
}

struct RingBufferInner<T> {
    state: Mutex<RingBufferState<T>>,
    write_notify: Condvar,
}

struct RingBufferState<T> {
    buf: Vec<Option<T>>,
    head: usize,  // virtual counter (wraps)
    tail: usize,  // virtual counter (wraps)
    close_write: bool,
    close_err: Option<Arc<dyn Error + Send + Sync>>,
}
```

**Key Methods:**

| Method | Signature | Description |
|--------|-----------|-------------|
| `new` | `fn new(capacity: usize) -> Self` | Create with capacity |
| `write` | `fn write(&self, data: &[T]) -> Result<usize, BufferError>` | Write (overwrites oldest) |
| `add` | `fn add(&self, item: T) -> Result<(), BufferError>` | Add single (overwrites) |

## Error Types

```rust
#[derive(Debug, Clone)]
pub enum BufferError {
    Closed,
    ClosedWithError(Arc<dyn Error + Send + Sync>),
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct Done;
```

## Convenience Functions

```rust
// Growable buffers
pub fn bytes() -> Buffer<u8>         // 1KB
pub fn bytes_1kb() -> Buffer<u8>     // 1KB
pub fn bytes_4kb() -> Buffer<u8>     // 4KB
pub fn bytes_16kb() -> Buffer<u8>    // 16KB
pub fn bytes_64kb() -> Buffer<u8>    // 64KB
pub fn bytes_256b() -> Buffer<u8>    // 256B

// Blocking buffers
pub fn block_bytes() -> BlockBuffer<u8>      // 1KB
pub fn block_bytes_1kb() -> BlockBuffer<u8>  // 1KB
pub fn block_bytes_4kb() -> BlockBuffer<u8>  // 4KB
pub fn block_bytes_16kb() -> BlockBuffer<u8> // 16KB
pub fn block_bytes_64kb() -> BlockBuffer<u8> // 64KB

// Ring buffers
pub fn ring_bytes(size: usize) -> RingBuffer<u8>
pub fn ring_bytes_1kb() -> RingBuffer<u8>    // 1KB
pub fn ring_bytes_4kb() -> RingBuffer<u8>    // 4KB
pub fn ring_bytes_16kb() -> RingBuffer<u8>   // 16KB
pub fn ring_bytes_64kb() -> RingBuffer<u8>   // 64KB
```

## Thread Safety

All types implement `Send + Sync` and support `Clone`:

```rust
// Clone shares the underlying buffer via Arc
let buf = Buffer::<i32>::new();
let buf_clone = buf.clone();  // Same underlying buffer

// Safe to send to other threads
std::thread::spawn(move || {
    buf_clone.add(42).unwrap();
});
```

## Usage Patterns

### Producer-Consumer

```rust
use giztoy_buffer::{BlockBuffer, Done};
use std::thread;

let buf = BlockBuffer::<i32>::new(4);
let producer_buf = buf.clone();

let producer = thread::spawn(move || {
    for i in 0..100 {
        producer_buf.add(i).unwrap();
    }
    producer_buf.close_write().unwrap();
});

let mut collected = Vec::new();
loop {
    match buf.next() {
        Ok(item) => collected.push(item),
        Err(Done) => break,
    }
}

producer.join().unwrap();
```

### Sliding Window

```rust
use giztoy_buffer::RingBuffer;

let buf = RingBuffer::<f32>::new(100);

// Write more than capacity - old data overwritten
for i in 0..200 {
    buf.add(i as f32).unwrap();
}

// Buffer contains only last 100 values
assert_eq!(buf.len(), 100);
let window = buf.to_vec();  // [100.0, 101.0, ..., 199.0]
```

## Implementation Details

### VecDeque vs Vec

- **Buffer**: Uses `VecDeque<T>` for O(1) `pop_front()`
- **BlockBuffer/RingBuffer**: Use `Vec<Option<T>>` for circular buffer

### Wrapping Arithmetic

RingBuffer uses `wrapping_add` for counters to handle overflow:

```rust
state.tail = state.tail.wrapping_add(1);
if state.tail.wrapping_sub(state.head) > capacity {
    state.head = state.head.wrapping_add(1);
}
```

### Dual Condvar Pattern (BlockBuffer)

BlockBuffer uses two Condvars for precise signaling:

```rust
not_full: Condvar,   // Signals writers when space available
not_empty: Condvar,  // Signals readers when data available
```

## Differences from Go Implementation

| Aspect | Go | Rust |
|--------|----|----- |
| Internal storage | `[]T` slice | `Vec<Option<T>>` or `VecDeque<T>` |
| `Buffer.Next()` | LIFO (pops from end) | FIFO (pops from front) |
| `Bytes()` / `to_vec()` | Returns internal slice | Returns copy |
| Cloning | Not supported | Via `Arc` (shared) |
| Error type | `error` interface | `BufferError` enum |
| Default impl | Via interface | Via `Default` trait |
