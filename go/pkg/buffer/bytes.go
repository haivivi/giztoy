package buffer

var (
	_ BytesBuffer = (*BlockBuffer[byte])(nil)
	_ BytesBuffer = (*Buffer[byte])(nil)
	_ BytesBuffer = (*RingBuffer[byte])(nil)
)

// BytesBuffer defines the interface for a buffer that can be read and written to.
// It provides a common abstraction over different buffer implementations for byte data.
// All implementations are thread-safe and support concurrent access.
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

// Bytes16KB creates a new BlockBuffer with 16KB capacity.
func Bytes16KB() *BlockBuffer[byte] {
	return BlockN[byte](1 << 14)
}

// Bytes4KB creates a new BlockBuffer with 4KB capacity.
func Bytes4KB() *BlockBuffer[byte] {
	return BlockN[byte](1 << 12)
}

// Bytes1KB creates a new BlockBuffer with 1KB capacity.
func Bytes1KB() *BlockBuffer[byte] {
	return BlockN[byte](1 << 10)
}

// Bytes256B creates a new BlockBuffer with 256 bytes capacity.
func Bytes256B() *BlockBuffer[byte] {
	return BlockN[byte](1 << 8)
}

// Bytes creates a new growable Buffer with 1KB initial capacity.
func Bytes() *Buffer[byte] {
	return N[byte](1 << 10)
}

// BytesRing creates a new RingBuffer with the specified capacity.
func BytesRing(size int) *RingBuffer[byte] {
	return RingN[byte](size)
}
