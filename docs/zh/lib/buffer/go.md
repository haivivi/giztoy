# Buffer Package - Go Implementation

Import: `github.com/haivivi/giztoy/pkg/buffer`

📚 [Go Documentation](https://pkg.go.dev/github.com/haivivi/giztoy/pkg/buffer)

## Types

### Buffer[T]

Growable buffer with generic type support.

```go
type Buffer[T any] struct {
    writeNotify chan struct{}
    mu          sync.Mutex
    closeWrite  bool
    closeErr    error
    buf         []T
}
```

**Key Methods:**

| Method | Signature | Description |
|--------|-----------|-------------|
| `N` | `func N[T any](n int) *Buffer[T]` | Create with initial capacity |
| `Write` | `(b *Buffer[T]) Write(p []T) (int, error)` | Append elements |
| `Read` | `(b *Buffer[T]) Read(p []T) (int, error)` | Read elements (blocks) |
| `Add` | `(b *Buffer[T]) Add(t T) error` | Add single element |
| `Next` | `(b *Buffer[T]) Next() (T, error)` | Pop from end (LIFO) |
| `Bytes` | `(b *Buffer[T]) Bytes() []T` | Get internal slice (unsafe) |

### BlockBuffer[T]

Fixed-size circular buffer with blocking semantics.

```go
type BlockBuffer[T any] struct {
    cond       *sync.Cond
    mu         sync.Mutex
    buf        []T
    head, tail int64
    closeWrite bool
    closeErr   error
}
```

**Key Methods:**

| Method | Signature | Description |
|--------|-----------|-------------|
| `Block` | `func Block[T any](buf []T) *BlockBuffer[T]` | Create from existing slice |
| `BlockN` | `func BlockN[T any](size int) *BlockBuffer[T]` | Create with size |
| `Write` | `(bb *BlockBuffer[T]) Write(p []T) (int, error)` | Write (blocks when full) |
| `Read` | `(bb *BlockBuffer[T]) Read(p []T) (int, error)` | Read (blocks when empty) |
| `Next` | `(bb *BlockBuffer[T]) Next() (T, error)` | Read single (FIFO) |

### RingBuffer[T]

Fixed-size circular buffer with overwrite semantics.

```go
type RingBuffer[T any] struct {
    writeNotify chan struct{}
    mu          sync.Mutex
    buf         []T
    head, tail  int64
    closeWrite  bool
    closeErr    error
}
```

**Key Methods:**

| Method | Signature | Description |
|--------|-----------|-------------|
| `RingN` | `func RingN[T any](size int) *RingBuffer[T]` | Create with size |
| `Write` | `(rb *RingBuffer[T]) Write(p []T) (int, error)` | Write (overwrites oldest) |
| `Add` | `(rb *RingBuffer[T]) Add(t T) error` | Add single (overwrites) |

### BytesBuffer Interface

Common interface for byte buffers:

```go
type BytesBuffer interface {
    Write(p []byte) (n int, err error)
    Read(p []byte) (n int, err error)
    Discard(n int) (err error)
    Close() error
    CloseWrite() error
    CloseWithError(err error) error
    Error() error
    Reset()
    Bytes() []byte
    Len() int
}
```

### Convenience Functions

```go
func Bytes16KB() *BlockBuffer[byte]  // 16KB blocking buffer
func Bytes4KB() *BlockBuffer[byte]   // 4KB blocking buffer
func Bytes1KB() *BlockBuffer[byte]   // 1KB blocking buffer
func Bytes256B() *BlockBuffer[byte]  // 256B blocking buffer
func Bytes() *Buffer[byte]           // 1KB growable buffer
func BytesRing(size int) *RingBuffer[byte]  // Ring buffer
```

## Error Handling

```go
var ErrIteratorDone = errors.New("iterator done")
```

- `ErrIteratorDone`: Returned by `Next()` when buffer is closed and empty
- `io.EOF`: Returned by `Read()` when buffer is closed and empty  
- `io.ErrClosedPipe`: Default error for closed buffers

## Usage Patterns

### Producer-Consumer with BlockBuffer

```go
buf := buffer.Bytes4KB()

// Producer goroutine
go func() {
    for data := range source {
        _, err := buf.Write(data)
        if err != nil {
            return
        }
    }
    buf.CloseWrite()
}()

// Consumer goroutine
tmp := make([]byte, 1024)
for {
    n, err := buf.Read(tmp)
    if err == io.EOF {
        break
    }
    process(tmp[:n])
}
```

### Sliding Window with RingBuffer

```go
buf := buffer.RingN[float64](100)  // Keep last 100 samples

// Streaming producer
go func() {
    for sample := range stream {
        buf.Add(sample)  // Overwrites oldest when full
    }
    buf.CloseWrite()
}()

// Periodic consumer
ticker := time.NewTicker(time.Second)
for range ticker.C {
    samples := buf.Bytes()  // Get current window
    average := computeAverage(samples)
}
```

### Iterator Pattern

```go
buf := buffer.N[Event](100)

// Using Next() for iteration
for {
    event, err := buf.Next()
    if errors.Is(err, buffer.ErrIteratorDone) {
        break
    }
    if err != nil {
        log.Error(err)
        break
    }
    handleEvent(event)
}
```

## Implementation Details

### Circular Buffer Arithmetic

BlockBuffer and RingBuffer use virtual counters for head/tail:

```go
// Position in physical buffer
pos := head % int64(len(buf))

// Available data
available := tail - head

// Check if full (BlockBuffer only)
isFull := tail - head == int64(len(buf))
```

### Notification Mechanism

- **Buffer**: Uses buffered channel `make(chan struct{}, 1)` for non-blocking notification
- **BlockBuffer**: Uses `sync.Cond` for precise signal/broadcast control
- **RingBuffer**: Uses buffered channel (same as Buffer)

### Lock Patterns

All types use `sync.Mutex` with deferred unlock:

```go
func (b *Buffer[T]) Read(p []T) (n int, err error) {
    b.mu.Lock()
    defer b.mu.Unlock()
    
    // Wait loop with unlock/relock
    for len(b.buf) == 0 {
        if b.closeWrite {
            return 0, io.EOF
        }
        b.mu.Unlock()
        <-b.writeNotify  // Wait for notification
        b.mu.Lock()
        // Re-check state after relock
    }
    // ... read logic
}
```
