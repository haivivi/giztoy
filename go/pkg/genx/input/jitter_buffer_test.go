package input

import (
	"testing"
)

// testPacket is a simple packet type for testing.
type testPacket struct {
	data []byte
	ts   int64
}

func (p testPacket) Timestamp() int64 { return p.ts }

func TestJitterBuffer_Basic(t *testing.T) {
	jb := NewJitterBuffer[int64, testPacket](100)

	// Push packets out of order
	jb.Push(testPacket{data: []byte("third"), ts: 300})
	jb.Push(testPacket{data: []byte("first"), ts: 100})
	jb.Push(testPacket{data: []byte("second"), ts: 200})

	if jb.Len() != 3 {
		t.Errorf("expected len 3, got %d", jb.Len())
	}

	// Pop should return in timestamp order
	pkt, ok := jb.Pop()
	if !ok || pkt.ts != 100 || string(pkt.data) != "first" {
		t.Errorf("expected first packet (ts=100), got ts=%d", pkt.ts)
	}

	pkt, ok = jb.Pop()
	if !ok || pkt.ts != 200 || string(pkt.data) != "second" {
		t.Errorf("expected second packet (ts=200), got ts=%d", pkt.ts)
	}

	pkt, ok = jb.Pop()
	if !ok || pkt.ts != 300 || string(pkt.data) != "third" {
		t.Errorf("expected third packet (ts=300), got ts=%d", pkt.ts)
	}

	// Buffer should be empty
	_, ok = jb.Pop()
	if ok {
		t.Error("expected empty buffer")
	}
}

func TestJitterBuffer_Peek(t *testing.T) {
	jb := NewJitterBuffer[int64, testPacket](100)

	// Peek on empty buffer
	_, ok := jb.Peek()
	if ok {
		t.Error("expected false for empty buffer")
	}

	jb.Push(testPacket{data: []byte("b"), ts: 200})
	jb.Push(testPacket{data: []byte("a"), ts: 100})

	// Peek should return smallest without removing
	pkt, ok := jb.Peek()
	if !ok || pkt.ts != 100 {
		t.Errorf("expected peek ts=100, got ts=%d", pkt.ts)
	}

	// Len should still be 2
	if jb.Len() != 2 {
		t.Errorf("expected len 2 after peek, got %d", jb.Len())
	}
}

func TestJitterBuffer_MaxItems(t *testing.T) {
	jb := NewJitterBuffer[int64, testPacket](3)

	// Push 5 packets
	for i := int64(1); i <= 5; i++ {
		jb.Push(testPacket{data: []byte{byte(i)}, ts: i * 100})
	}

	// Should only have 3 (oldest dropped)
	if jb.Len() != 3 {
		t.Errorf("expected len 3, got %d", jb.Len())
	}

	// The remaining should be the 3 with largest timestamps
	// After heap operations, we pop in order
	pkt, _ := jb.Pop()
	if pkt.ts != 300 {
		t.Errorf("expected ts=300, got ts=%d", pkt.ts)
	}
	pkt, _ = jb.Pop()
	if pkt.ts != 400 {
		t.Errorf("expected ts=400, got ts=%d", pkt.ts)
	}
	pkt, _ = jb.Pop()
	if pkt.ts != 500 {
		t.Errorf("expected ts=500, got ts=%d", pkt.ts)
	}
}

func TestJitterBuffer_Clear(t *testing.T) {
	jb := NewJitterBuffer[int64, testPacket](100)

	jb.Push(testPacket{data: []byte("a"), ts: 100})
	jb.Push(testPacket{data: []byte("b"), ts: 200})

	jb.Clear()

	if jb.Len() != 0 {
		t.Errorf("expected len 0 after clear, got %d", jb.Len())
	}

	_, ok := jb.Pop()
	if ok {
		t.Error("expected empty buffer after clear")
	}
}

func TestJitterBuffer_DuplicateTimestamps(t *testing.T) {
	jb := NewJitterBuffer[int64, testPacket](100)

	// Push packets with same timestamp
	jb.Push(testPacket{data: []byte("a"), ts: 100})
	jb.Push(testPacket{data: []byte("b"), ts: 100})
	jb.Push(testPacket{data: []byte("c"), ts: 100})

	if jb.Len() != 3 {
		t.Errorf("expected len 3, got %d", jb.Len())
	}

	// All should pop with ts=100
	for i := 0; i < 3; i++ {
		pkt, ok := jb.Pop()
		if !ok || pkt.ts != 100 {
			t.Errorf("expected ts=100, got ts=%d, ok=%v", pkt.ts, ok)
		}
	}
}

func TestJitterBuffer_OutOfOrderBurst(t *testing.T) {
	jb := NewJitterBuffer[int64, testPacket](100)

	// Simulate out-of-order burst: 5, 3, 1, 4, 2
	timestamps := []int64{500, 300, 100, 400, 200}
	for _, ts := range timestamps {
		jb.Push(testPacket{data: []byte{byte(ts / 100)}, ts: ts})
	}

	// Pop should be in order: 1, 2, 3, 4, 5
	expected := []int64{100, 200, 300, 400, 500}
	for _, exp := range expected {
		pkt, ok := jb.Pop()
		if !ok || pkt.ts != exp {
			t.Errorf("expected ts=%d, got ts=%d", exp, pkt.ts)
		}
	}
}
