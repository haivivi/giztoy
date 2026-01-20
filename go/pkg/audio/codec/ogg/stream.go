package ogg

/*
#include <ogg/ogg.h>
#include <stdlib.h>
#include <string.h>

// Wrapper functions to avoid CGO pointer issues

static ogg_stream_state* alloc_stream_state() {
    return (ogg_stream_state*)calloc(1, sizeof(ogg_stream_state));
}

static void free_stream_state(ogg_stream_state *state) {
    if (state) {
        ogg_stream_clear(state);
        free(state);
    }
}

static int stream_init(ogg_stream_state *state, int serialno) {
    return ogg_stream_init(state, serialno);
}

static void stream_reset(ogg_stream_state *state) {
    ogg_stream_reset(state);
}

static void stream_reset_serialno(ogg_stream_state *state, int serialno) {
    ogg_stream_reset_serialno(state, serialno);
}

static int stream_pagein(ogg_stream_state *state, ogg_page *page) {
    return ogg_stream_pagein(state, page);
}

static int stream_packetout(ogg_stream_state *state, ogg_packet *packet) {
    return ogg_stream_packetout(state, packet);
}

static int stream_packetpeek(ogg_stream_state *state, ogg_packet *packet) {
    return ogg_stream_packetpeek(state, packet);
}

static int stream_eos(ogg_stream_state *state) {
    return ogg_stream_eos(state);
}

static int stream_packetin(ogg_stream_state *state, ogg_packet *packet) {
    return ogg_stream_packetin(state, packet);
}

static int stream_pageout(ogg_stream_state *state, ogg_page *page) {
    return ogg_stream_pageout(state, page);
}

static int stream_pageout_fill(ogg_stream_state *state, ogg_page *page, int fillbytes) {
    return ogg_stream_pageout_fill(state, page, fillbytes);
}

static int stream_flush(ogg_stream_state *state, ogg_page *page) {
    return ogg_stream_flush(state, page);
}

static int stream_flush_fill(ogg_stream_state *state, ogg_page *page, int fillbytes) {
    return ogg_stream_flush_fill(state, page, fillbytes);
}

// Get page data as copies
static long get_page_header_len(ogg_page *page) {
    return page->header_len;
}

static long get_page_body_len(ogg_page *page) {
    return page->body_len;
}

static void copy_page_header(ogg_page *page, unsigned char *dst) {
    memcpy(dst, page->header, page->header_len);
}

static void copy_page_body(ogg_page *page, unsigned char *dst) {
    memcpy(dst, page->body, page->body_len);
}

// Get packet data
static long get_packet_bytes(ogg_packet *packet) {
    return packet->bytes;
}

static void copy_packet_data(ogg_packet *packet, unsigned char *dst) {
    memcpy(dst, packet->packet, packet->bytes);
}

static ogg_int64_t get_packet_granulepos(ogg_packet *packet) {
    return packet->granulepos;
}

static ogg_int64_t get_packet_packetno(ogg_packet *packet) {
    return packet->packetno;
}

static int get_packet_bos(ogg_packet *packet) {
    return packet->b_o_s;
}

static int get_packet_eos(ogg_packet *packet) {
    return packet->e_o_s;
}
*/
import "C"
import (
	"errors"
	"runtime"
	"sync/atomic"
	"unsafe"
)

var (
	// ErrStream indicates a stream error.
	ErrStream = errors.New("ogg: stream error")
	// ErrNoPacket indicates no packet is available.
	ErrNoPacket = errors.New("ogg: no packet available")
	// ErrHole indicates a gap in the data (packet loss).
	ErrHole = errors.New("ogg: hole in data")
)

// StreamState manages the encoding/decoding of a logical Ogg bitstream.
// Must call Clear() when done to release resources.
type StreamState struct {
	state    *C.ogg_stream_state
	serialNo int32
	page     C.ogg_page
	packet   C.ogg_packet
	cleared  atomic.Bool
	cleanup  runtime.Cleanup
}

// freeStreamState releases C resources.
func freeStreamState(ptr uintptr) {
	C.free_stream_state((*C.ogg_stream_state)(unsafe.Pointer(ptr)))
}

// NewStreamState creates a new stream state with the given serial number.
// Returns an error if memory allocation fails.
func NewStreamState(serialNo int32) (*StreamState, error) {
	state := C.alloc_stream_state()
	if state == nil {
		return nil, errors.New("ogg: failed to allocate stream state")
	}
	s := &StreamState{
		state:    state,
		serialNo: serialNo,
	}
	C.stream_init(s.state, C.int(serialNo))
	s.cleanup = runtime.AddCleanup(s, freeStreamState, uintptr(unsafe.Pointer(state)))
	return s, nil
}

// Clear releases resources. Safe to call multiple times.
func (s *StreamState) Clear() {
	if s.cleared.CompareAndSwap(false, true) {
		s.cleanup.Stop()
		C.free_stream_state(s.state)
		s.state = nil
	}
}

// Reset resets the stream state.
func (s *StreamState) Reset() {
	C.stream_reset(s.state)
}

// ResetSerialNo resets and changes the serial number.
func (s *StreamState) ResetSerialNo(serialNo int32) {
	C.stream_reset_serialno(s.state, C.int(serialNo))
	s.serialNo = serialNo
}

// SerialNo returns the stream serial number.
func (s *StreamState) SerialNo() int32 {
	return s.serialNo
}

// PageIn submits a page to the stream for packetization.
func (s *StreamState) PageIn(page *Page) error {
	if C.stream_pagein(s.state, &page.page) != 0 {
		return ErrStream
	}
	return nil
}

// PacketOut extracts a packet from the stream.
// Returns the packet and nil on success.
// Returns ErrNoPacket if no complete packet is available.
// Returns ErrHole if there's a gap in the data.
func (s *StreamState) PacketOut(packet *Packet) error {
	result := C.stream_packetout(s.state, &s.packet)
	switch result {
	case 1:
		// Copy packet data
		packet.data = make([]byte, C.get_packet_bytes(&s.packet))
		if len(packet.data) > 0 {
			C.copy_packet_data(&s.packet, (*C.uchar)(unsafe.Pointer(&packet.data[0])))
		}
		packet.granulePos = int64(C.get_packet_granulepos(&s.packet))
		packet.packetNo = int64(C.get_packet_packetno(&s.packet))
		packet.bos = C.get_packet_bos(&s.packet) != 0
		packet.eos = C.get_packet_eos(&s.packet) != 0
		return nil
	case 0:
		return ErrNoPacket
	default:
		return ErrHole
	}
}

// PacketPeek peeks at the next packet without removing it.
func (s *StreamState) PacketPeek(packet *Packet) error {
	result := C.stream_packetpeek(s.state, &s.packet)
	switch result {
	case 1:
		packet.data = make([]byte, C.get_packet_bytes(&s.packet))
		if len(packet.data) > 0 {
			C.copy_packet_data(&s.packet, (*C.uchar)(unsafe.Pointer(&packet.data[0])))
		}
		packet.granulePos = int64(C.get_packet_granulepos(&s.packet))
		packet.packetNo = int64(C.get_packet_packetno(&s.packet))
		packet.bos = C.get_packet_bos(&s.packet) != 0
		packet.eos = C.get_packet_eos(&s.packet) != 0
		return nil
	case 0:
		return ErrNoPacket
	default:
		return ErrHole
	}
}

// EOS returns true if the last packet was marked end of stream.
func (s *StreamState) EOS() bool {
	return C.stream_eos(s.state) != 0
}

// --- Encoding functions ---

// PacketIn submits a packet for page generation.
// The data is copied internally. Returns an error if data is empty.
func (s *StreamState) PacketIn(data []byte, granulePos, packetNo int64, bos, eos bool) error {
	if len(data) == 0 {
		return errors.New("ogg: empty packet data")
	}

	var cPacket C.ogg_packet

	// Allocate and copy data to C memory
	cData := C.malloc(C.size_t(len(data)))
	if cData == nil {
		return errors.New("ogg: malloc failed")
	}
	C.memcpy(cData, unsafe.Pointer(&data[0]), C.size_t(len(data)))

	cPacket.packet = (*C.uchar)(cData)
	cPacket.bytes = C.long(len(data))
	cPacket.granulepos = C.ogg_int64_t(granulePos)
	cPacket.packetno = C.ogg_int64_t(packetNo)
	if bos {
		cPacket.b_o_s = 1
	}
	if eos {
		cPacket.e_o_s = 1
	}

	result := C.stream_packetin(s.state, &cPacket)
	C.free(cData)

	if result != 0 {
		return ErrStream
	}
	return nil
}

// PageOut generates a page from submitted packets.
// Returns the page data and nil on success.
// Returns ErrNoPacket if no complete page is available.
func (s *StreamState) PageOut() (header, body []byte, err error) {
	if C.stream_pageout(s.state, &s.page) == 0 {
		return nil, nil, ErrNoPacket
	}

	headerLen := int(C.get_page_header_len(&s.page))
	bodyLen := int(C.get_page_body_len(&s.page))

	header = make([]byte, headerLen)
	body = make([]byte, bodyLen)

	if headerLen > 0 {
		C.copy_page_header(&s.page, (*C.uchar)(unsafe.Pointer(&header[0])))
	}
	if bodyLen > 0 {
		C.copy_page_body(&s.page, (*C.uchar)(unsafe.Pointer(&body[0])))
	}

	return header, body, nil
}

// PageOutFill generates a page with a specific fill level.
func (s *StreamState) PageOutFill(fillBytes int) (header, body []byte, err error) {
	if C.stream_pageout_fill(s.state, &s.page, C.int(fillBytes)) == 0 {
		return nil, nil, ErrNoPacket
	}

	headerLen := int(C.get_page_header_len(&s.page))
	bodyLen := int(C.get_page_body_len(&s.page))

	header = make([]byte, headerLen)
	body = make([]byte, bodyLen)

	if headerLen > 0 {
		C.copy_page_header(&s.page, (*C.uchar)(unsafe.Pointer(&header[0])))
	}
	if bodyLen > 0 {
		C.copy_page_body(&s.page, (*C.uchar)(unsafe.Pointer(&body[0])))
	}

	return header, body, nil
}

// Flush forces remaining packets into a page.
func (s *StreamState) Flush() (header, body []byte, err error) {
	if C.stream_flush(s.state, &s.page) == 0 {
		return nil, nil, ErrNoPacket
	}

	headerLen := int(C.get_page_header_len(&s.page))
	bodyLen := int(C.get_page_body_len(&s.page))

	header = make([]byte, headerLen)
	body = make([]byte, bodyLen)

	if headerLen > 0 {
		C.copy_page_header(&s.page, (*C.uchar)(unsafe.Pointer(&header[0])))
	}
	if bodyLen > 0 {
		C.copy_page_body(&s.page, (*C.uchar)(unsafe.Pointer(&body[0])))
	}

	return header, body, nil
}

// FlushFill forces remaining packets into a page with a specific fill level.
func (s *StreamState) FlushFill(fillBytes int) (header, body []byte, err error) {
	if C.stream_flush_fill(s.state, &s.page, C.int(fillBytes)) == 0 {
		return nil, nil, ErrNoPacket
	}

	headerLen := int(C.get_page_header_len(&s.page))
	bodyLen := int(C.get_page_body_len(&s.page))

	header = make([]byte, headerLen)
	body = make([]byte, bodyLen)

	if headerLen > 0 {
		C.copy_page_header(&s.page, (*C.uchar)(unsafe.Pointer(&header[0])))
	}
	if bodyLen > 0 {
		C.copy_page_body(&s.page, (*C.uchar)(unsafe.Pointer(&body[0])))
	}

	return header, body, nil
}
