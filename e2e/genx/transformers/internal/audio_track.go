package internal

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/haivivi/giztoy/go/pkg/audio/codec/mp3"
	"github.com/haivivi/giztoy/go/pkg/audio/resampler"
	"github.com/haivivi/giztoy/go/pkg/genx"
)

// StreamKey uniquely identifies a logical stream.
// (role, mimetype, streamid) forms the unique tuple.
type StreamKey struct {
	Role     genx.Role
	MIMEType string
	StreamID string
}

// AudioTrack collects audio from a conversation stream.
// Audio is grouped by (role, mimetype, streamid) tuple.
// On Save, audio is output in order: for each StreamID, user audio then model audio.
type AudioTrack struct {
	outputPath string
	sampleRate int
	channels   int

	mu            sync.Mutex
	streams       map[StreamKey]*bytes.Buffer // Each stream's audio buffer
	streamIDOrder []string                    // Order of StreamIDs as they first appear
	seenStreamIDs map[string]bool             // Track which StreamIDs we've seen
}

// NewAudioTrack creates a new AudioTrack that writes to outputPath.
func NewAudioTrack(outputPath string, sampleRate, channels int) *AudioTrack {
	return &AudioTrack{
		outputPath:    outputPath,
		sampleRate:    sampleRate,
		channels:      channels,
		streams:       make(map[StreamKey]*bytes.Buffer),
		seenStreamIDs: make(map[string]bool),
	}
}

// HandleChunk processes a message chunk.
// Audio is stored in buffers grouped by (role, mimetype, streamid).
func (t *AudioTrack) HandleChunk(chunk *genx.MessageChunk) {
	if chunk == nil {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	// Get StreamID from chunk
	streamID := ""
	if chunk.Ctrl != nil {
		streamID = chunk.Ctrl.StreamID
	}

	// Track StreamID order (when first seen)
	if streamID != "" && !t.seenStreamIDs[streamID] {
		t.seenStreamIDs[streamID] = true
		t.streamIDOrder = append(t.streamIDOrder, streamID)
		fmt.Printf("  [Track] stream %s started (role=%s)\n", streamID[:8], chunk.Role)
	}

	// Skip BOS markers (just control signals)
	if chunk.IsBeginOfStream() {
		return
	}

	// Handle audio data
	blob, ok := chunk.Part.(*genx.Blob)
	if !ok || len(blob.Data) == 0 {
		return
	}

	// Skip non-audio
	if !IsAudioMIME(blob.MIMEType) {
		return
	}

	// Convert to target PCM format
	pcm, err := t.toPCM(blob.Data, blob.MIMEType, chunk.Role)
	if err != nil || len(pcm) == 0 {
		return
	}

	// Store in appropriate buffer by (role, mimetype, streamid)
	key := StreamKey{
		Role:     chunk.Role,
		MIMEType: "audio/pcm", // All audio is converted to PCM
		StreamID: streamID,
	}

	buf := t.streams[key]
	if buf == nil {
		buf = &bytes.Buffer{}
		t.streams[key] = buf
	}
	buf.Write(pcm)
}

// toPCM converts audio data to PCM at the target sample rate.
func (t *AudioTrack) toPCM(data []byte, mimeType string, role genx.Role) ([]byte, error) {
	if len(data) == 0 {
		return nil, nil
	}

	// Detect actual format from data
	isMP3 := len(data) >= 2 && data[0] == 0xFF && (data[1]&0xE0) == 0xE0

	var pcm []byte
	var srcRate, srcChannels int

	if isMP3 {
		// Decode MP3 to PCM
		var err error
		pcm, srcRate, srcChannels, err = mp3.DecodeFull(bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("mp3 decode: %w", err)
		}
	} else {
		// Assume PCM - determine source format based on role
		pcm = data
		if role == genx.RoleUser {
			// TTS outputs 16kHz mono
			srcRate = 16000
			srcChannels = 1
		} else {
			// DashScope outputs 24kHz mono
			srcRate = 24000
			srcChannels = 1
		}
	}

	// Handle missing format info
	if srcRate == 0 {
		srcRate = t.sampleRate
	}
	if srcChannels == 0 {
		srcChannels = t.channels
	}

	// Resample if needed
	if srcRate != t.sampleRate || srcChannels != t.channels {
		resampled, err := t.resamplePCM(pcm, srcRate, srcChannels)
		if err != nil {
			return nil, err
		}
		pcm = resampled
	}

	return pcm, nil
}

// resamplePCM resamples PCM data to the target format.
func (t *AudioTrack) resamplePCM(pcm []byte, srcRate, srcChannels int) ([]byte, error) {
	srcFmt := resampler.Format{SampleRate: srcRate, Stereo: srcChannels == 2}
	dstFmt := resampler.Format{SampleRate: t.sampleRate, Stereo: t.channels == 2}

	rs, err := resampler.New(bytes.NewReader(pcm), srcFmt, dstFmt)
	if err != nil {
		return nil, err
	}
	defer rs.Close()

	var out bytes.Buffer
	_, err = io.Copy(&out, rs)
	if err != nil && err != io.EOF {
		return nil, err
	}
	return out.Bytes(), nil
}

// Save writes the collected audio to the output file as MP3.
// Audio is ordered: for each StreamID (in order of first appearance),
// output user audio then model audio.
func (t *AudioTrack) Save() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Merge audio in correct order: user -> model for each StreamID
	var allData bytes.Buffer

	for _, streamID := range t.streamIDOrder {
		// User audio for this StreamID
		userKey := StreamKey{Role: genx.RoleUser, MIMEType: "audio/pcm", StreamID: streamID}
		if buf := t.streams[userKey]; buf != nil && buf.Len() > 0 {
			allData.Write(buf.Bytes())
		}

		// Model audio for this StreamID
		modelKey := StreamKey{Role: genx.RoleModel, MIMEType: "audio/pcm", StreamID: streamID}
		if buf := t.streams[modelKey]; buf != nil && buf.Len() > 0 {
			allData.Write(buf.Bytes())
		}
	}

	// Also include any audio without StreamID (legacy fallback)
	for key, buf := range t.streams {
		if key.StreamID == "" && buf.Len() > 0 {
			allData.Write(buf.Bytes())
		}
	}

	if allData.Len() == 0 {
		return nil
	}

	// Encode to MP3
	f, err := os.Create(t.outputPath)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = mp3.EncodePCMStream(f, &allData, t.sampleRate, t.channels)
	return err
}

// Duration returns the duration of collected audio in seconds.
func (t *AudioTrack) Duration() float64 {
	t.mu.Lock()
	defer t.mu.Unlock()

	totalBytes := 0
	for _, buf := range t.streams {
		totalBytes += buf.Len()
	}

	// PCM: sampleRate * channels * 2 bytes/sample
	bytesPerSecond := t.sampleRate * t.channels * 2
	return float64(totalBytes) / float64(bytesPerSecond)
}

// TeeToTrack wraps a stream and copies all chunks to the track.
func TeeToTrack(src genx.Stream, track *AudioTrack) genx.Stream {
	return &teeTrackStream{src: src, track: track}
}

type teeTrackStream struct {
	src   genx.Stream
	track *AudioTrack
}

func (s *teeTrackStream) Next() (*genx.MessageChunk, error) {
	chunk, err := s.src.Next()
	if err != nil {
		return nil, err
	}
	if chunk != nil {
		s.track.HandleChunk(chunk)
	}
	return chunk, nil
}

func (s *teeTrackStream) Close() error {
	return s.src.Close()
}

func (s *teeTrackStream) CloseWithError(err error) error {
	return s.src.CloseWithError(err)
}
