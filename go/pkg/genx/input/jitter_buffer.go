package input

import (
	"cmp"
	"container/heap"
)

// Timestamped is a packet with a comparable timestamp.
type Timestamped[T cmp.Ordered] interface {
	Timestamp() T
}

// JitterBuffer reorders out-of-order packets by timestamp using a min-heap.
// It is generic over the timestamp type T and packet type P.
//
// Example usage:
//
//	type MyPacket struct {
//	    Data []byte
//	    TS   int64
//	}
//	func (p MyPacket) Timestamp() int64 { return p.TS }
//
//	jb := NewJitterBuffer[int64, MyPacket](100)
//	jb.Push(MyPacket{Data: data, TS: timestamp})
//	pkt, ok := jb.Pop()
type JitterBuffer[T cmp.Ordered, P Timestamped[T]] struct {
	heap     jitterHeap[T, P]
	maxItems int
}

// NewJitterBuffer creates a new JitterBuffer with the given maximum capacity.
// When the buffer exceeds maxItems, the oldest packets are dropped.
func NewJitterBuffer[T cmp.Ordered, P Timestamped[T]](maxItems int) *JitterBuffer[T, P] {
	return &JitterBuffer[T, P]{
		maxItems: maxItems,
	}
}

// Push adds a packet to the buffer, maintaining heap order by timestamp.
// If the buffer exceeds maxItems, the oldest packet is dropped.
func (b *JitterBuffer[T, P]) Push(pkt P) {
	heap.Push(&b.heap, pkt)

	// Trim if over capacity
	for b.heap.Len() > b.maxItems {
		heap.Pop(&b.heap)
	}
}

// Pop returns and removes the packet with the smallest timestamp.
// Returns false if the buffer is empty.
func (b *JitterBuffer[T, P]) Pop() (P, bool) {
	if b.heap.Len() == 0 {
		var zero P
		return zero, false
	}
	return heap.Pop(&b.heap).(P), true
}

// Peek returns the packet with the smallest timestamp without removing it.
// Returns false if the buffer is empty.
func (b *JitterBuffer[T, P]) Peek() (P, bool) {
	if b.heap.Len() == 0 {
		var zero P
		return zero, false
	}
	return b.heap[0], true
}

// Len returns the number of packets in the buffer.
func (b *JitterBuffer[T, P]) Len() int {
	return b.heap.Len()
}

// Clear removes all packets from the buffer.
func (b *JitterBuffer[T, P]) Clear() {
	b.heap = nil
}

// jitterHeap implements heap.Interface for generic packets.
type jitterHeap[T cmp.Ordered, P Timestamped[T]] []P

func (h jitterHeap[T, P]) Len() int           { return len(h) }
func (h jitterHeap[T, P]) Less(i, j int) bool { return h[i].Timestamp() < h[j].Timestamp() }
func (h jitterHeap[T, P]) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *jitterHeap[T, P]) Push(x any) {
	*h = append(*h, x.(P))
}

func (h *jitterHeap[T, P]) Pop() any {
	old := *h
	n := len(old)
	x := old[n-1]
	var zero P
	old[n-1] = zero // avoid memory leak
	*h = old[:n-1]
	return x
}
