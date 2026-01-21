package buffer

import (
	"bytes"
	"io"
	"testing"
)

func TestBytes16KB(t *testing.T) {
	buf := Bytes16KB()
	if buf == nil {
		t.Fatal("Bytes16KB returned nil")
	}

	// Verify capacity
	data := make([]byte, 1<<14)
	n, err := buf.Write(data)
	if err != nil {
		t.Fatalf("Write error: %v", err)
	}
	if n != 1<<14 {
		t.Fatalf("Write returned %d, want %d", n, 1<<14)
	}
	buf.CloseWrite()
}

func TestBytes4KB(t *testing.T) {
	buf := Bytes4KB()
	if buf == nil {
		t.Fatal("Bytes4KB returned nil")
	}

	// Verify capacity
	data := make([]byte, 1<<12)
	n, err := buf.Write(data)
	if err != nil {
		t.Fatalf("Write error: %v", err)
	}
	if n != 1<<12 {
		t.Fatalf("Write returned %d, want %d", n, 1<<12)
	}
	buf.CloseWrite()
}

func TestBytes1KB(t *testing.T) {
	buf := Bytes1KB()
	if buf == nil {
		t.Fatal("Bytes1KB returned nil")
	}

	// Verify capacity
	data := make([]byte, 1<<10)
	n, err := buf.Write(data)
	if err != nil {
		t.Fatalf("Write error: %v", err)
	}
	if n != 1<<10 {
		t.Fatalf("Write returned %d, want %d", n, 1<<10)
	}
	buf.CloseWrite()
}

func TestBytes256B(t *testing.T) {
	buf := Bytes256B()
	if buf == nil {
		t.Fatal("Bytes256B returned nil")
	}

	// Verify capacity
	data := make([]byte, 1<<8)
	n, err := buf.Write(data)
	if err != nil {
		t.Fatalf("Write error: %v", err)
	}
	if n != 1<<8 {
		t.Fatalf("Write returned %d, want %d", n, 1<<8)
	}
	buf.CloseWrite()
}

func TestBytes(t *testing.T) {
	buf := Bytes()
	if buf == nil {
		t.Fatal("Bytes returned nil")
	}

	// Growable buffer should accept more than initial capacity
	data := make([]byte, 2048)
	for i := range data {
		data[i] = byte(i % 256)
	}

	n, err := buf.Write(data)
	if err != nil {
		t.Fatalf("Write error: %v", err)
	}
	if n != 2048 {
		t.Fatalf("Write returned %d, want 2048", n)
	}
	buf.CloseWrite()

	// Read all data back
	got := make([]byte, 4096)
	n, err = buf.Read(got)
	if err != nil {
		t.Fatalf("Read error: %v", err)
	}
	if n != 2048 {
		t.Fatalf("Read returned %d, want 2048", n)
	}
	if !bytes.Equal(got[:n], data) {
		t.Fatal("Read data mismatch")
	}
}

func TestBytesRing(t *testing.T) {
	buf := BytesRing(100)
	if buf == nil {
		t.Fatal("BytesRing returned nil")
	}

	// RingBuffer should overwrite old data when full
	data := make([]byte, 150)
	for i := range data {
		data[i] = byte(i)
	}

	n, err := buf.Write(data)
	if err != nil {
		t.Fatalf("Write error: %v", err)
	}
	if n != 150 {
		t.Fatalf("Write returned %d, want 150", n)
	}

	// Should only have the last 100 bytes
	if buf.Len() != 100 {
		t.Fatalf("Len() = %d, want 100", buf.Len())
	}

	buf.CloseWrite()

	got, err := io.ReadAll(buf)
	if err != nil {
		t.Fatalf("ReadAll error: %v", err)
	}
	if len(got) != 100 {
		t.Fatalf("ReadAll returned %d bytes, want 100", len(got))
	}
}

// Verify BytesBuffer interface implementation
func TestBytesBufferInterface(t *testing.T) {
	testCases := []struct {
		name string
		buf  BytesBuffer
	}{
		{"BlockBuffer", Bytes1KB()},
		{"Buffer", Bytes()},
		{"RingBuffer", BytesRing(100)},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			buf := tc.buf

			// Test Write
			n, err := buf.Write([]byte{1, 2, 3})
			if err != nil {
				t.Fatalf("Write error: %v", err)
			}
			if n != 3 {
				t.Fatalf("Write returned %d, want 3", n)
			}

			// Test Len
			if buf.Len() != 3 {
				t.Fatalf("Len() = %d, want 3", buf.Len())
			}

			// Test Bytes
			b := buf.Bytes()
			if !bytes.Equal(b, []byte{1, 2, 3}) {
				t.Fatalf("Bytes() = %v, want [1,2,3]", b)
			}

			// Test Reset
			buf.Reset()
			if buf.Len() != 0 {
				t.Fatalf("Len() after Reset = %d, want 0", buf.Len())
			}

			// Test Error before close
			if buf.Error() != nil {
				t.Fatalf("Error() before close = %v, want nil", buf.Error())
			}

			// Test CloseWrite
			buf.Write([]byte{4, 5, 6})
			if err := buf.CloseWrite(); err != nil {
				t.Fatalf("CloseWrite error: %v", err)
			}

			// Test Read
			got := make([]byte, 10)
			n, err = buf.Read(got)
			if err != nil {
				t.Fatalf("Read error: %v", err)
			}
			if n != 3 {
				t.Fatalf("Read returned %d, want 3", n)
			}

			// Test Close
			if err := buf.Close(); err != nil {
				t.Fatalf("Close error: %v", err)
			}
		})
	}
}
