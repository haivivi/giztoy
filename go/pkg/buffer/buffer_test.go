package buffer

import (
	"bytes"
	"errors"
	"io"
	"sync"
	"testing"
	"time"
)

func TestBuffer_WriteRead(t *testing.T) {
	buf := N[byte](10)

	// Write data
	n, err := buf.Write([]byte{1, 2, 3, 4, 5})
	if err != nil {
		t.Fatalf("Write error: %v", err)
	}
	if n != 5 {
		t.Fatalf("Write returned %d, want 5", n)
	}

	// Verify length
	if buf.Len() != 5 {
		t.Fatalf("Len() = %d, want 5", buf.Len())
	}

	// Close write to allow reads to complete
	buf.CloseWrite()

	// Read data
	got := make([]byte, 10)
	n, err = buf.Read(got)
	if err != nil {
		t.Fatalf("Read error: %v", err)
	}
	if n != 5 {
		t.Fatalf("Read returned %d, want 5", n)
	}
	if !bytes.Equal(got[:n], []byte{1, 2, 3, 4, 5}) {
		t.Fatalf("Read got %v, want [1,2,3,4,5]", got[:n])
	}

	// Next read should return EOF
	_, err = buf.Read(got)
	if err != io.EOF {
		t.Fatalf("expected EOF, got %v", err)
	}
}

func TestBuffer_ConcurrentWriteRead(t *testing.T) {
	buf := N[byte](100)

	data := make([]byte, 256)
	for i := range data {
		data[i] = byte(i)
	}

	var wg sync.WaitGroup
	wg.Add(2)

	// Writer goroutine
	go func() {
		defer wg.Done()
		for i := 0; i < len(data); i += 32 {
			end := i + 32
			if end > len(data) {
				end = len(data)
			}
			_, err := buf.Write(data[i:end])
			if err != nil {
				t.Errorf("Write error: %v", err)
				return
			}
		}
		buf.CloseWrite()
	}()

	// Reader goroutine
	var received []byte
	go func() {
		defer wg.Done()
		tmp := make([]byte, 64)
		for {
			n, err := buf.Read(tmp)
			if err != nil {
				if err == io.EOF {
					break
				}
				t.Errorf("Read error: %v", err)
				return
			}
			received = append(received, tmp[:n]...)
		}
	}()

	wg.Wait()

	if !bytes.Equal(received, data) {
		t.Errorf("received data mismatch")
	}
}

func TestBuffer_Add(t *testing.T) {
	buf := N[int](10)

	for i := 0; i < 5; i++ {
		if err := buf.Add(i); err != nil {
			t.Fatalf("Add(%d) error: %v", i, err)
		}
	}

	if buf.Len() != 5 {
		t.Fatalf("Len() = %d, want 5", buf.Len())
	}
}

func TestBuffer_Next(t *testing.T) {
	buf := N[int](10)

	// Add some items
	for i := 1; i <= 3; i++ {
		buf.Add(i)
	}
	buf.CloseWrite()

	// Next reads from the beginning (FIFO order)
	v, err := buf.Next()
	if err != nil {
		t.Fatalf("Next error: %v", err)
	}
	if v != 1 {
		t.Fatalf("Next() = %d, want 1", v)
	}

	v, err = buf.Next()
	if err != nil {
		t.Fatalf("Next error: %v", err)
	}
	if v != 2 {
		t.Fatalf("Next() = %d, want 2", v)
	}

	v, err = buf.Next()
	if err != nil {
		t.Fatalf("Next error: %v", err)
	}
	if v != 3 {
		t.Fatalf("Next() = %d, want 3", v)
	}

	// Next should return ErrIteratorDone
	_, err = buf.Next()
	if !errors.Is(err, ErrIteratorDone) {
		t.Fatalf("expected ErrIteratorDone, got %v", err)
	}
}

func TestBuffer_Discard(t *testing.T) {
	buf := N[byte](10)
	buf.Write([]byte{1, 2, 3, 4, 5})

	// Discard 2 bytes
	if err := buf.Discard(2); err != nil {
		t.Fatalf("Discard error: %v", err)
	}

	if buf.Len() != 3 {
		t.Fatalf("Len() = %d, want 3", buf.Len())
	}

	// Discard more than available
	if err := buf.Discard(100); err != nil {
		t.Fatalf("Discard error: %v", err)
	}

	if buf.Len() != 0 {
		t.Fatalf("Len() = %d, want 0", buf.Len())
	}
}

func TestBuffer_Reset(t *testing.T) {
	buf := N[byte](10)
	buf.Write([]byte{1, 2, 3, 4, 5})

	buf.Reset()

	if buf.Len() != 0 {
		t.Fatalf("Len() = %d, want 0", buf.Len())
	}
}

func TestBuffer_Bytes(t *testing.T) {
	buf := N[byte](10)
	buf.Write([]byte{1, 2, 3})

	b := buf.Bytes()
	if !bytes.Equal(b, []byte{1, 2, 3}) {
		t.Fatalf("Bytes() = %v, want [1,2,3]", b)
	}
}

func TestBuffer_CloseWithError(t *testing.T) {
	buf := N[byte](10)
	buf.Write([]byte{1, 2, 3})

	customErr := errors.New("custom error")
	buf.CloseWithError(customErr)

	if buf.Error() != customErr {
		t.Fatalf("Error() = %v, want %v", buf.Error(), customErr)
	}

	// Write should fail
	_, err := buf.Write([]byte{4, 5})
	if err == nil {
		t.Fatal("Write should fail after CloseWithError")
	}
	if !errors.Is(err, customErr) {
		t.Fatalf("Write error should wrap customErr, got %v", err)
	}

	// Read should fail
	tmp := make([]byte, 10)
	_, err = buf.Read(tmp)
	if err == nil {
		t.Fatal("Read should fail after CloseWithError")
	}
}

func TestBuffer_Close(t *testing.T) {
	buf := N[byte](10)
	buf.Write([]byte{1, 2, 3})

	buf.Close()

	// Write should fail with ErrClosedPipe
	_, err := buf.Write([]byte{4, 5})
	if err == nil {
		t.Fatal("Write should fail after Close")
	}
	if !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("Write error should wrap ErrClosedPipe, got %v", err)
	}
}

func TestBuffer_CloseWriteThenRead(t *testing.T) {
	buf := N[byte](10)
	buf.Write([]byte{1, 2, 3})
	buf.CloseWrite()

	// Should still be able to read existing data
	got := make([]byte, 10)
	n, err := buf.Read(got)
	if err != nil {
		t.Fatalf("Read error: %v", err)
	}
	if n != 3 {
		t.Fatalf("Read returned %d, want 3", n)
	}

	// Next read should return EOF
	_, err = buf.Read(got)
	if err != io.EOF {
		t.Fatalf("expected EOF, got %v", err)
	}

	// Write should fail
	_, err = buf.Write([]byte{4})
	if err == nil {
		t.Fatal("Write should fail after CloseWrite")
	}
}

func TestBuffer_BlockingRead(t *testing.T) {
	buf := N[byte](10)

	done := make(chan struct{})
	go func() {
		tmp := make([]byte, 5)
		n, err := buf.Read(tmp)
		if err != nil {
			t.Errorf("Read error: %v", err)
			return
		}
		if n != 3 {
			t.Errorf("Read returned %d, want 3", n)
		}
		close(done)
	}()

	// Give reader time to block
	time.Sleep(50 * time.Millisecond)

	// Write data to unblock reader
	buf.Write([]byte{1, 2, 3})
	buf.CloseWrite()

	select {
	case <-done:
		// Success
	case <-time.After(time.Second):
		t.Fatal("Read did not unblock")
	}
}

func TestBuffer_AddToClosedBuffer(t *testing.T) {
	buf := N[int](10)
	buf.CloseWrite()

	err := buf.Add(1)
	if err == nil {
		t.Fatal("Add should fail after CloseWrite")
	}
}

func TestBuffer_DiscardFromClosedBuffer(t *testing.T) {
	buf := N[byte](10)
	buf.Write([]byte{1, 2, 3})

	customErr := errors.New("closed")
	buf.CloseWithError(customErr)

	err := buf.Discard(1)
	if err == nil {
		t.Fatal("Discard should fail after CloseWithError")
	}
}

func TestBuffer_NextFromClosedBuffer(t *testing.T) {
	buf := N[int](10)
	buf.Add(1)

	customErr := errors.New("closed")
	buf.CloseWithError(customErr)

	_, err := buf.Next()
	if err == nil {
		t.Fatal("Next should fail after CloseWithError")
	}
}

func TestBuffer_DoubleCloseWrite(t *testing.T) {
	buf := N[byte](10)

	err1 := buf.CloseWrite()
	err2 := buf.CloseWrite()

	if err1 != nil {
		t.Fatalf("First CloseWrite error: %v", err1)
	}
	if err2 != nil {
		t.Fatalf("Second CloseWrite error: %v", err2)
	}
}

func TestBuffer_DoubleCloseWithError(t *testing.T) {
	buf := N[byte](10)

	err1 := errors.New("error1")
	err2 := errors.New("error2")

	buf.CloseWithError(err1)
	buf.CloseWithError(err2)

	// First error should be retained
	if buf.Error() != err1 {
		t.Fatalf("Error() = %v, want %v", buf.Error(), err1)
	}
}
