package buffer

import (
	"bytes"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"strconv"
	"testing"
)

func TestBlockBuffer(t *testing.T) {
	t.Run("size=1", func(t *testing.T) {
		rb := BlockN[int](1)
		closed := make(chan struct{})
		producerErr := make(chan error, 1)
		go func() {
			n, err := rb.Write([]int{1, 2, 3})
			if err != nil {
				producerErr <- fmt.Errorf("write [1,2,3] with error: %w", err)
				return
			}
			if n != 3 {
				producerErr <- fmt.Errorf("write [1,2,3] with n=%d", n)
				return
			}

			<-closed

			if _, err := rb.Write([]int{4}); err == nil {
				producerErr <- errors.New("write [4] expected error, but got nil")
				return
			}

			if err := rb.Add(5); err == nil {
				producerErr <- errors.New("add 5 expected error, but got nil")
				return
			}

			producerErr <- nil
		}()

		var got [1]int
		n, err := rb.Read(got[:])
		if err != nil {
			t.Errorf("read with error: %v", err)
		}
		if n != 1 {
			t.Errorf("read with n=%d", n)
		}
		if got != [1]int{1} {
			t.Errorf("got=%v", got)
		}
		v, err := rb.Next()
		if err != nil {
			t.Errorf("next with error: %v", err)
		}
		if v != 2 {
			t.Errorf("next with v=%d", v)
		}
		n, err = rb.Read(got[:])
		if err != nil {
			t.Errorf("read with error: %v", err)
		}
		if n != 1 {
			t.Errorf("read with n=%d", n)
		}
		if got != [1]int{3} {
			t.Errorf("got=%v", got)
		}
		if err := rb.CloseWrite(); err != nil {
			t.Errorf("close write with error: %v", err)
		}
		close(closed)
		if err := <-producerErr; err != nil {
			t.Fatal(err)
		}
	})

	t.Run("size=2", func(t *testing.T) {
		rb := BlockN[int](2)
		closed := make(chan struct{})
		producerErr := make(chan error, 1)
		go func() {
			n, err := rb.Write([]int{1, 2, 3, 4})
			if err != nil {
				producerErr <- fmt.Errorf("write [1,2,3,4] with error: %w", err)
				return
			}
			if n != 4 {
				producerErr <- fmt.Errorf("write [1,2,3,4] with n=%d", n)
				return
			}

			<-closed

			if _, err := rb.Write([]int{5}); err == nil {
				producerErr <- errors.New("write [5] expected error, but got nil")
				return
			}

			if err := rb.Add(5); err == nil {
				producerErr <- errors.New("add 5 expected error, but got nil")
				return
			}

			producerErr <- nil
		}()

		var got [2]int
		n, err := rb.Read(got[:])
		if err != nil {
			t.Errorf("read with error: %v", err)
		}
		if n != 2 {
			t.Errorf("read with n=%d", n)
		}
		if got != [2]int{1, 2} {
			t.Errorf("got=%v", got)
		}
		v, err := rb.Next()
		if err != nil {
			t.Errorf("next with error: %v", err)
		}
		if v != 3 {
			t.Errorf("next with v=%d", v)
		}
		got = [2]int{}
		n, err = rb.Read(got[:])
		if err != nil {
			t.Errorf("read with error: %v", err)
		}
		if n != 1 {
			t.Errorf("read with n=%d", n)
		}
		if got != [2]int{4} {
			t.Errorf("got=%v", got)
		}
		if err := rb.CloseWrite(); err != nil {
			t.Errorf("close write with error: %v", err)
		}
		close(closed)
		if err := <-producerErr; err != nil {
			t.Fatal(err)
		}
	})

	for i := 1; i <= 4096; i *= 2 {
		sz := i
		t.Run("large.size="+strconv.Itoa(i), func(t *testing.T) {
			rb := BlockN[byte](sz)

			data := make([]byte, 10240)
			rand.Read(data)
			go func() {
				for i := 0; i < len(data); {
					randLen := int(data[i]) + 537
					if i+randLen > len(data) {
						randLen = len(data) - i
					}
					n, err := rb.Write(data[i : i+randLen])
					if err != nil {
						rb.CloseWithError(err)
						return
					}
					if n != randLen {
						rb.CloseWithError(fmt.Errorf("write with n=%d", n))
						return
					}
					i += randLen
				}
				rb.CloseWrite()
			}()

			buf := make([]byte, 537)
			ptr := 0
			for {
				n, err := rb.Read(buf)
				if err != nil {
					if errors.Is(err, io.EOF) {
						break
					}
					t.Fatalf("read with error: %v", err)
				}
				if n == 0 {
					t.Fatal("read with n=0")
				}
				if !bytes.Equal(buf[:n], data[ptr:ptr+n]) {
					t.Fatalf("read with data not equal")
				}
				ptr += n
			}
		})
	}

}

func BenchmarkBlockBuffer(b *testing.B) {
	rb := BlockN[byte](4096)
	data := make([]byte, 102400)
	rand.Read(data)
	go func() {
		for i := 0; i < len(data); {
			randLen := int(data[i]) + 537
			if i+randLen > len(data) {
				randLen = len(data) - i
			}
			n, err := rb.Write(data[i : i+randLen])
			if err != nil {
				b.Errorf("write with error: %v", err)
			}
			if n != randLen {
				b.Errorf("write with n=%d", n)
			}
			i += randLen
		}
		rb.CloseWrite()
	}()

	buf := make([]byte, 537)
	ptr := 0
	for {
		n, err := rb.Read(buf)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			b.Fatalf("read with error: %v", err)
		}
		if n == 0 {
			b.Fatal("read with n=0")
		}
		if !bytes.Equal(buf[:n], data[ptr:ptr+n]) {
			b.Fatalf("read with data not equal")
		}
		ptr += n
	}
}
