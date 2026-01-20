package ogg

import (
	"bytes"
	"io"
	"testing"
)

func TestSyncState(t *testing.T) {
	sync := NewSyncState()
	defer sync.Clear()

	// Write some data (not valid Ogg, just testing the interface)
	data := []byte("test data")
	n, err := sync.Write(data)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if n != len(data) {
		t.Errorf("Write returned %d, want %d", n, len(data))
	}

	// Try to extract a page (should fail with need more)
	var page Page
	err = sync.PageOut(&page)
	if err != ErrNeedMore && err != ErrSync {
		t.Errorf("PageOut returned unexpected error: %v", err)
	}
}

func TestStreamState(t *testing.T) {
	stream := NewStreamState(12345)
	defer stream.Clear()

	if stream.SerialNo() != 12345 {
		t.Errorf("SerialNo() = %d, want 12345", stream.SerialNo())
	}

	stream.Reset()

	if stream.EOS() {
		t.Error("EOS() should be false initially")
	}
}

func TestEncoderDecoder(t *testing.T) {
	// Test data
	packets := [][]byte{
		[]byte("header packet"),
		[]byte("data packet 1"),
		[]byte("data packet 2"),
		[]byte("final packet"),
	}

	// Encode
	var buf bytes.Buffer
	enc, err := NewEncoder(&buf)
	if err != nil {
		t.Fatalf("NewEncoder failed: %v", err)
	}

	// Write BOS packet
	if err := enc.WritePacket(packets[0], 0, true, false); err != nil {
		t.Fatalf("WritePacket BOS failed: %v", err)
	}

	// Write data packets
	for i, pkt := range packets[1 : len(packets)-1] {
		if err := enc.WritePacket(pkt, int64((i+1)*100), false, false); err != nil {
			t.Fatalf("WritePacket %d failed: %v", i+1, err)
		}
	}

	// Write EOS packet
	if err := enc.WritePacket(packets[len(packets)-1], int64(len(packets)*100), false, true); err != nil {
		t.Fatalf("WritePacket EOS failed: %v", err)
	}

	if err := enc.Close(); err != nil {
		t.Fatalf("Encoder Close failed: %v", err)
	}

	t.Logf("Encoded %d bytes", buf.Len())

	// Decode
	dec := NewDecoder(&buf)
	defer dec.Close()

	stream := NewStreamState(enc.SerialNo())
	defer stream.Clear()

	var decoded [][]byte
	var packet Packet

	for {
		page, err := dec.ReadPage()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("ReadPage failed: %v", err)
		}

		if err := stream.PageIn(page); err != nil {
			t.Fatalf("PageIn failed: %v", err)
		}

		for {
			err := stream.PacketOut(&packet)
			if err == ErrNoPacket {
				break
			}
			if err != nil {
				t.Fatalf("PacketOut failed: %v", err)
			}
			decoded = append(decoded, packet.Data())
		}
	}

	// Verify
	if len(decoded) != len(packets) {
		t.Errorf("Got %d packets, want %d", len(decoded), len(packets))
	}

	for i, pkt := range decoded {
		if i < len(packets) && !bytes.Equal(pkt, packets[i]) {
			t.Errorf("Packet %d: got %q, want %q", i, pkt, packets[i])
		}
	}
}

func TestPacketWriter(t *testing.T) {
	var buf bytes.Buffer
	// NewPacketWriter takes granule increment (e.g., 960 for 20ms at 48kHz)
	pw, err := NewPacketWriter(&buf, 960)
	if err != nil {
		t.Fatalf("NewPacketWriter failed: %v", err)
	}

	// Write header
	header := []byte("OpusHead")
	if err := pw.WriteHeader(header); err != nil {
		t.Fatalf("WriteHeader failed: %v", err)
	}

	// Write data packets
	for i := 0; i < 5; i++ {
		data := []byte{byte(i), byte(i + 1), byte(i + 2)}
		if err := pw.Write(data); err != nil {
			t.Fatalf("Write %d failed: %v", i, err)
		}
	}

	// Write EOS
	if err := pw.WriteEOS([]byte("final")); err != nil {
		t.Fatalf("WriteEOS failed: %v", err)
	}

	// Flush and close
	if err := pw.Flush(); err != nil {
		t.Fatalf("Flush failed: %v", err)
	}
	if err := pw.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Verify serial number
	if pw.SerialNo() == 0 {
		t.Error("SerialNo should not be 0")
	}

	// Verify output is valid OGG
	data := buf.Bytes()
	if len(data) < 4 {
		t.Fatalf("Output too short: %d bytes", len(data))
	}
	if string(data[:4]) != "OggS" {
		t.Errorf("Invalid OGG magic: got %q", string(data[:4]))
	}
}

func TestEncoderWithSerial(t *testing.T) {
	var buf bytes.Buffer
	enc := NewEncoderWithSerial(&buf, 54321)

	if enc.SerialNo() != 54321 {
		t.Errorf("SerialNo() = %d, want 54321", enc.SerialNo())
	}

	enc.Close()
}

func TestPageHelpers(t *testing.T) {
	// Create an OGG file first
	var buf bytes.Buffer
	enc, _ := NewEncoder(&buf)
	enc.WritePacket([]byte("test"), 100, true, true)
	enc.Close()

	// Read the page
	dec := NewDecoder(&buf)
	defer dec.Close()

	page, err := dec.ReadPage()
	if err != nil {
		t.Fatalf("ReadPage failed: %v", err)
	}

	// Test Page helper methods
	header := page.Header()
	if len(header) == 0 {
		t.Error("Header() returned empty")
	}

	body := page.Body()
	if len(body) == 0 {
		t.Error("Body() returned empty")
	}

	serialNo := page.SerialNo()
	t.Logf("SerialNo: %d", serialNo)

	pageNo := page.PageNo()
	t.Logf("PageNo: %d", pageNo)

	isBOS := page.IsBOS()
	t.Logf("IsBOS: %v", isBOS)

	isEOS := page.IsEOS()
	t.Logf("IsEOS: %v", isEOS)

	granulePos := page.GranulePos()
	t.Logf("GranulePos: %d", granulePos)

	packets := page.Packets()
	t.Logf("Packets: %d", packets)
}

func TestPacketHelpers(t *testing.T) {
	// Create OGG with packets
	var buf bytes.Buffer
	enc, _ := NewEncoder(&buf)
	testData := []byte("test packet data")
	enc.WritePacket(testData, 500, true, true)
	enc.Close()

	// Read and parse
	dec := NewDecoder(&buf)
	defer dec.Close()

	page, _ := dec.ReadPage()
	stream := NewStreamState(page.SerialNo())
	defer stream.Clear()

	stream.PageIn(page)

	var packet Packet
	if err := stream.PacketOut(&packet); err != nil {
		t.Fatalf("PacketOut failed: %v", err)
	}

	// Test Packet helper methods
	data := packet.Data()
	if !bytes.Equal(data, testData) {
		t.Errorf("Data() = %q, want %q", data, testData)
	}

	byteLen := packet.Bytes()
	if byteLen != int64(len(testData)) {
		t.Errorf("Bytes() = %d, want %d", byteLen, len(testData))
	}

	bos := packet.BOS()
	t.Logf("BOS: %v", bos)

	eos := packet.EOS()
	t.Logf("EOS: %v", eos)

	granulePos := packet.GranulePos()
	t.Logf("GranulePos: %d", granulePos)

	packetNo := packet.PacketNo()
	t.Logf("PacketNo: %d", packetNo)
}

func TestStreamResetSerialNo(t *testing.T) {
	stream := NewStreamState(100)
	defer stream.Clear()

	if stream.SerialNo() != 100 {
		t.Errorf("Initial SerialNo() = %d, want 100", stream.SerialNo())
	}

	stream.ResetSerialNo(200)

	if stream.SerialNo() != 200 {
		t.Errorf("After Reset, SerialNo() = %d, want 200", stream.SerialNo())
	}
}

func TestSyncStateBuffer(t *testing.T) {
	sync := NewSyncState()
	defer sync.Clear()

	// Get buffer and write to it
	buf := sync.Buffer(100)
	if buf == nil {
		t.Fatal("Buffer() returned nil")
	}
	if len(buf) != 100 {
		t.Errorf("Buffer length = %d, want 100", len(buf))
	}

	// Write some data
	copy(buf, []byte("hello"))
	if err := sync.Wrote(5); err != nil {
		t.Errorf("Wrote() failed: %v", err)
	}
}

func TestSyncStateReset(t *testing.T) {
	sync := NewSyncState()
	defer sync.Clear()

	// Write some data
	sync.Write([]byte("test data"))

	// Reset should clear the state
	sync.Reset()

	// Should be able to write again
	_, err := sync.Write([]byte("new data"))
	if err != nil {
		t.Errorf("Write after Reset failed: %v", err)
	}
}

func TestEncoderFlush(t *testing.T) {
	var buf bytes.Buffer
	enc, _ := NewEncoder(&buf)

	// Write a packet
	enc.WritePacket([]byte("test"), 100, true, false)

	// Flush should force page output
	if err := enc.Flush(); err != nil {
		t.Fatalf("Flush failed: %v", err)
	}

	// Buffer should have data now
	if buf.Len() == 0 {
		t.Error("Buffer empty after Flush")
	}

	enc.Close()
}

func TestMultipleClearCalls(t *testing.T) {
	// Test that Clear() is idempotent
	sync := NewSyncState()
	sync.Clear()
	sync.Clear() // Should not panic

	stream := NewStreamState(100)
	stream.Clear()
	stream.Clear() // Should not panic
}
