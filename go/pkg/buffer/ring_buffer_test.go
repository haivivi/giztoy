package buffer

import (
	"bytes"
	"io"
	"testing"
)

func TestRingBuffer(t *testing.T) {
	t.Run("size=1", func(t *testing.T) {
		rb := RingN[byte](1)
		rb.Write([]byte{1, 2, 3})
		rb.CloseWrite()

		if rb.Len() != 1 {
			t.Errorf("len=%d", rb.Len())
		}

		got, err := io.ReadAll(rb)
		if err != nil {
			t.Errorf("read with error: %v", err)
		}
		if !bytes.Equal(got, []byte{3}) {
			t.Errorf("got=%v", got)
		}
	})

	t.Run("size=2", func(t *testing.T) {
		rb := RingN[byte](2)
		rb.Write([]byte{1, 2, 3})
		rb.CloseWrite()

		if rb.Len() != 2 {
			t.Errorf("len=%d", rb.Len())
		}

		got, err := io.ReadAll(rb)
		if err != nil {
			t.Errorf("read with error: %v", err)
		}
		if !bytes.Equal(got, []byte{2, 3}) {
			t.Errorf("got=%v", got)
		}
	})

	t.Run("size=3", func(t *testing.T) {
		rb := RingN[byte](3)
		rb.Write([]byte{1, 2, 3})
		rb.CloseWrite()

		if rb.Len() != 3 {
			t.Errorf("len=%d", rb.Len())
		}

		got, err := io.ReadAll(rb)
		if err != nil {
			t.Errorf("read with error: %v", err)
		}
		if !bytes.Equal(got, []byte{1, 2, 3}) {
			t.Errorf("got=%v", got)
		}
	})

	t.Run("size=4", func(t *testing.T) {
		rb := RingN[byte](4)
		rb.Write([]byte{1, 2, 3})
		rb.CloseWrite()

		if rb.Len() != 3 {
			t.Errorf("len=%d", rb.Len())
		}

		got, err := io.ReadAll(rb)
		if err != nil {
			t.Errorf("read with error: %v", err)
		}
		if !bytes.Equal(got, []byte{1, 2, 3}) {
			t.Errorf("got=%v", got)
		}
	})

	t.Run("size=100,7,1", func(t *testing.T) {
		rb := RingN[byte](7)
		for i := range 100 {
			rb.Write([]byte{byte(i)})
		}
		rb.CloseWrite()

		if rb.Len() != 7 {
			t.Errorf("len=%d", rb.Len())
		}

		got, err := io.ReadAll(rb)
		if err != nil {
			t.Errorf("read with error: %v", err)
		}
		if !bytes.Equal(got, []byte{93, 94, 95, 96, 97, 98, 99}) {
			t.Errorf("got=%v", got)
		}
	})

	t.Run("size=100,7,3", func(t *testing.T) {
		rb := RingN[byte](7)
		for i := range 100 {
			rb.Write([]byte{byte(i), byte(i + 1), byte(i + 2)})
		}
		rb.CloseWrite()

		if rb.Len() != 7 {
			t.Errorf("len=%d", rb.Len())
		}

		got, err := io.ReadAll(rb)
		if err != nil {
			t.Errorf("read with error: %v", err)
		}
		if !bytes.Equal(got, []byte{99, 98, 99, 100, 99, 100, 101}) {
			t.Errorf("got=%v", got)
		}
	})

	t.Run("size=100,7,7", func(t *testing.T) {
		rb := RingN[byte](7)
		for i := range 100 {
			rb.Write([]byte{byte(i), byte(i + 1), byte(i + 2), byte(i + 3), byte(i + 4), byte(i + 5), byte(i + 6)})
		}
		rb.CloseWrite()

		if rb.Len() != 7 {
			t.Errorf("len=%d", rb.Len())
		}

		got, err := io.ReadAll(rb)
		if err != nil {
			t.Errorf("read with error: %v", err)
		}
		if !bytes.Equal(got, []byte{99, 100, 101, 102, 103, 104, 105}) {
			t.Errorf("got=%v", got)
		}
	})

}
