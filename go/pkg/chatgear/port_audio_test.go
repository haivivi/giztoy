package chatgear

import (
	"io"
	"math"
	"testing"
	"time"

	"github.com/haivivi/giztoy/go/pkg/audio/codec/opus"
	"github.com/haivivi/giztoy/go/pkg/audio/pcm"
)

// =============================================================================
// Opus Roundtrip Tests
// =============================================================================

// generateSineWave creates a sine wave PCM signal for testing
func generateSineWave(samples int, freq float64, sampleRate int) []int16 {
	result := make([]int16, samples)
	for i := 0; i < samples; i++ {
		t := float64(i) / float64(sampleRate)
		value := math.Sin(2 * math.Pi * freq * t)
		result[i] = int16(value * 16000) // Use 50% of max amplitude
	}
	return result
}

// pcmCorrelation calculates the normalized cross-correlation between two signals
// It finds the best correlation by trying different delays to account for codec delay.
// Returns the maximum correlation value found.
func pcmCorrelation(a, b []int16) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}

	// Try different delays to find max correlation (codec introduces delay)
	maxCorr := 0.0
	for delay := 0; delay < 200; delay++ {
		corr := correlationAtDelay(a, b, delay)
		if corr > maxCorr {
			maxCorr = corr
		}
	}
	return maxCorr
}

// correlationAtDelay calculates correlation with a given delay
func correlationAtDelay(a, b []int16, delay int) float64 {
	n := len(a) - delay
	if len(b) < n {
		n = len(b)
	}
	if n <= 0 {
		return 0
	}

	var sumAB float64
	var sumA2, sumB2 float64

	for i := 0; i < n; i++ {
		fa := float64(a[i+delay])
		fb := float64(b[i])
		sumAB += fa * fb
		sumA2 += fa * fa
		sumB2 += fb * fb
	}

	denom := math.Sqrt(sumA2 * sumB2)
	if denom == 0 {
		return 0
	}
	return sumAB / denom
}

// pcmSNR calculates the Signal-to-Noise Ratio in dB, accounting for codec delay.
func pcmSNR(original, reconstructed []int16) float64 {
	if len(original) == 0 || len(reconstructed) == 0 {
		return 0
	}

	// Find best delay for SNR calculation
	bestSNR := -100.0
	for delay := 0; delay < 200; delay++ {
		snr := snrAtDelay(original, reconstructed, delay)
		if snr > bestSNR {
			bestSNR = snr
		}
	}
	return bestSNR
}

// snrAtDelay calculates SNR with a given delay
func snrAtDelay(original, reconstructed []int16, delay int) float64 {
	n := len(original) - delay
	if len(reconstructed) < n {
		n = len(reconstructed)
	}
	if n <= 0 {
		return -100
	}

	var signalPower, noisePower float64
	for i := 0; i < n; i++ {
		signal := float64(original[i+delay])
		noise := float64(original[i+delay] - reconstructed[i])
		signalPower += signal * signal
		noisePower += noise * noise
	}

	if noisePower == 0 {
		return 100 // Perfect reconstruction
	}

	return 10 * math.Log10(signalPower/noisePower)
}

func TestOpusEncoderDecoder_Roundtrip(t *testing.T) {
	// Test opus encode/decode directly without port layer
	sampleRate := 48000 // Use standard sample rate for opus
	channels := 1
	frameDuration := 20 * time.Millisecond
	frameSize := sampleRate * int(frameDuration.Milliseconds()) / 1000

	// Generate test signal: 440Hz sine wave
	inputPCM := generateSineWave(frameSize*5, 440, sampleRate)

	// Create encoder and decoder
	encoder, err := opus.NewVoIPEncoder(sampleRate, channels)
	if err != nil {
		t.Fatalf("NewVoIPEncoder: %v", err)
	}
	defer encoder.Close()

	decoder, err := opus.NewDecoder(sampleRate, channels)
	if err != nil {
		t.Fatalf("NewDecoder: %v", err)
	}
	defer decoder.Close()

	// Encode and decode frame by frame using DecodeTo for direct int16
	decodeBuf := make([]int16, frameSize*2) // Extra space
	var outputPCM []int16
	for i := 0; i < 5; i++ {
		start := i * frameSize
		end := start + frameSize
		frame := inputPCM[start:end]

		// Encode
		encoded, err := encoder.Encode(frame, frameSize)
		if err != nil {
			t.Fatalf("Encode frame %d: %v", i, err)
		}

		if len(encoded) == 0 {
			continue
		}

		// Decode using DecodeTo
		n, err := decoder.DecodeTo(encoded, decodeBuf)
		if err != nil {
			t.Fatalf("Decode frame %d: %v", i, err)
		}

		outputPCM = append(outputPCM, decodeBuf[:n*channels]...)
	}

	t.Logf("Input samples: %d, Output samples: %d", len(inputPCM), len(outputPCM))

	// Verify output
	if len(outputPCM) == 0 {
		t.Fatal("No output samples decoded")
	}

	// Calculate correlation (should be high for good codec)
	correlation := pcmCorrelation(inputPCM, outputPCM)
	t.Logf("Correlation: %.4f", correlation)

	// Opus is lossy, so we expect correlation > 0.9 for sine waves
	if correlation < 0.8 {
		t.Errorf("Correlation too low: %.4f (expected > 0.8)", correlation)
	}

	// Calculate SNR
	snr := pcmSNR(inputPCM, outputPCM)
	t.Logf("SNR: %.2f dB", snr)

	// Expect decent SNR for voice codec
	if snr < 10 {
		t.Errorf("SNR too low: %.2f dB (expected > 10)", snr)
	}
}

func TestOpusRoundtrip_ThroughPipe(t *testing.T) {
	// Test the full pipeline: Mic -> Encode -> Pipe -> Decode -> Speaker
	format := pcm.L16Mono24K
	sampleRate := format.SampleRate()
	frameDuration := 20 * time.Millisecond
	frameSize := int(format.SamplesInDuration(frameDuration))

	// Generate 3 frames of 440Hz sine wave
	inputPCM := generateSineWave(frameSize*3, 440, sampleRate)

	// Create mock mic that returns our test data
	mic := &roundtripMic{
		format:    format,
		data:      inputPCM,
		frameSize: frameSize,
	}

	// Create mock speaker to collect output
	speaker := &roundtripSpeaker{
		format: format,
	}

	// Create pipe for transport
	serverConn, clientConn := NewPipe()

	// Create client port
	clientPort := NewClientPort()

	// Start reading from mic and sending through pipe
	micDone := make(chan error, 1)
	go func() {
		micDone <- clientPort.ReadFromMic(mic)
	}()

	// Forward audio through pipe
	writeDone := make(chan error, 1)
	go func() {
		writeDone <- clientPort.WriteTo(clientConn)
	}()

	// On server side, decode and play to speaker
	decodeDone := make(chan error, 1)
	go func() {
		decoder, err := opus.NewDecoder(sampleRate, format.Channels())
		if err != nil {
			decodeDone <- err
			return
		}
		defer decoder.Close()

		decodeBuf := make([]int16, frameSize*2)
		for frame, err := range serverConn.OpusFrames() {
			if err != nil {
				decodeDone <- err
				return
			}

			n, err := decoder.DecodeTo(frame.Frame, decodeBuf)
			if err != nil {
				continue // Skip invalid frames
			}

			speaker.Write(decodeBuf[:n])

			// Stop after receiving enough
			if len(speaker.data) >= len(inputPCM) {
				break
			}
		}
		decodeDone <- nil
	}()

	// Wait for mic to finish (it will return EOF after all data is read)
	<-micDone

	// Give time for data to flow through
	time.Sleep(100 * time.Millisecond)

	// Close everything
	clientPort.Close()
	clientConn.Close()
	serverConn.Close()

	// Wait for goroutines
	<-writeDone
	<-decodeDone

	t.Logf("Input samples: %d, Output samples: %d", len(inputPCM), len(speaker.data))

	if len(speaker.data) == 0 {
		t.Fatal("No output samples received")
	}

	// Verify correlation
	correlation := pcmCorrelation(inputPCM, speaker.data)
	t.Logf("Correlation: %.4f", correlation)

	if correlation < 0.7 {
		t.Errorf("Correlation too low: %.4f (expected > 0.7)", correlation)
	}
}

// roundtripMic is a mock Mic for roundtrip testing
type roundtripMic struct {
	format    pcm.Format
	data      []int16
	frameSize int
	pos       int
}

func (m *roundtripMic) Read(buf []int16) (int, error) {
	if m.pos >= len(m.data) {
		return 0, io.EOF
	}

	n := copy(buf, m.data[m.pos:])
	m.pos += n
	return n, nil
}

func (m *roundtripMic) Format() pcm.Format {
	return m.format
}

// roundtripSpeaker is a mock Speaker for roundtrip testing
type roundtripSpeaker struct {
	format pcm.Format
	data   []int16
}

func (s *roundtripSpeaker) Write(buf []int16) (int, error) {
	s.data = append(s.data, buf...)
	return len(buf), nil
}

func (s *roundtripSpeaker) Format() pcm.Format {
	return s.format
}

func TestOpus_MultipleFrequencies(t *testing.T) {
	// Test with different frequencies to ensure codec handles variety
	sampleRate := 48000 // Use standard sample rate
	channels := 1
	frameDuration := 20 * time.Millisecond
	frameSize := sampleRate * int(frameDuration.Milliseconds()) / 1000

	encoder, err := opus.NewVoIPEncoder(sampleRate, channels)
	if err != nil {
		t.Fatalf("NewVoIPEncoder: %v", err)
	}
	defer encoder.Close()

	decoder, err := opus.NewDecoder(sampleRate, channels)
	if err != nil {
		t.Fatalf("NewDecoder: %v", err)
	}
	defer decoder.Close()

	frequencies := []float64{200, 440, 800, 1000, 2000}
	decodeBuf := make([]int16, frameSize*2)

	for _, freq := range frequencies {
		inputPCM := generateSineWave(frameSize*3, freq, sampleRate)
		var outputPCM []int16

		for i := 0; i < 3; i++ {
			start := i * frameSize
			end := start + frameSize
			frame := inputPCM[start:end]

			encoded, err := encoder.Encode(frame, frameSize)
			if err != nil {
				t.Fatalf("Encode at %dHz: %v", int(freq), err)
			}

			if len(encoded) == 0 {
				continue
			}

			n, err := decoder.DecodeTo(encoded, decodeBuf)
			if err != nil {
				t.Fatalf("Decode at %dHz: %v", int(freq), err)
			}

			outputPCM = append(outputPCM, decodeBuf[:n*channels]...)
		}

		correlation := pcmCorrelation(inputPCM, outputPCM)
		t.Logf("Frequency %dHz: correlation %.4f", int(freq), correlation)

		if correlation < 0.7 {
			t.Errorf("Low correlation at %dHz: %.4f", int(freq), correlation)
		}
	}
}

func TestOpus_SilenceHandling(t *testing.T) {
	// Test that silence is handled correctly (DTX - discontinuous transmission)
	sampleRate := 48000 // Use standard sample rate
	channels := 1
	frameDuration := 20 * time.Millisecond
	frameSize := sampleRate * int(frameDuration.Milliseconds()) / 1000

	encoder, err := opus.NewVoIPEncoder(sampleRate, channels)
	if err != nil {
		t.Fatalf("NewVoIPEncoder: %v", err)
	}
	defer encoder.Close()

	decoder, err := opus.NewDecoder(sampleRate, channels)
	if err != nil {
		t.Fatalf("NewDecoder: %v", err)
	}
	defer decoder.Close()

	// Create silent input
	silentPCM := make([]int16, frameSize*3)
	decodeBuf := make([]int16, frameSize*2)

	var encodedFrames int
	var outputPCM []int16

	for i := 0; i < 3; i++ {
		start := i * frameSize
		end := start + frameSize
		frame := silentPCM[start:end]

		encoded, err := encoder.Encode(frame, frameSize)
		if err != nil {
			t.Fatalf("Encode silence: %v", err)
		}

		if len(encoded) > 0 {
			encodedFrames++
			n, err := decoder.DecodeTo(encoded, decodeBuf)
			if err != nil {
				t.Fatalf("Decode silence: %v", err)
			}

			outputPCM = append(outputPCM, decodeBuf[:n*channels]...)
		}
	}

	t.Logf("Silence: %d frames encoded, %d samples output", encodedFrames, len(outputPCM))

	// Verify output is near-silent (may not be exactly zero due to codec artifacts)
	if len(outputPCM) > 0 {
		var maxAmp int16
		for _, s := range outputPCM {
			if s > maxAmp {
				maxAmp = s
			}
			if -s > maxAmp {
				maxAmp = -s
			}
		}
		t.Logf("Max amplitude in decoded silence: %d", maxAmp)

		// Expect low amplitude for silence
		if maxAmp > 1000 {
			t.Errorf("Decoded silence has high amplitude: %d", maxAmp)
		}
	}
}

// =============================================================================
// Full Audio Loopback Test
// =============================================================================

// TestAudioLoopback_FullPipeline tests the complete audio path:
// Mic -> ClientPort -> Pipe -> ServerPort -> Mixer -> streamAudioTo -> Pipe -> ClientPort -> Speaker
//
// Note: The mixer's streamAudioTo must be started AFTER track data is written
// because the mixer blocks on track.Read when a track exists but has no data.
// This is by design - the mixer waits for track data rather than outputting silence.
func TestAudioLoopback_FullPipeline(t *testing.T) {
	// Audio format
	format := pcm.L16Mono24K
	sampleRate := format.SampleRate()
	frameDuration := 20 * time.Millisecond
	frameSize := int(format.SamplesInDuration(frameDuration))

	// Generate test signal: 5 frames of 440Hz sine wave
	numFrames := 5
	inputPCM := generateSineWave(frameSize*numFrames, 440, sampleRate)

	// Create Mock Mic and Speaker
	mic := &loopbackMic{
		format:    format,
		data:      inputPCM,
		frameSize: frameSize,
	}
	speaker := &loopbackSpeaker{
		format: format,
	}

	// Create ports
	clientPort := NewClientPort()
	serverPort := NewServerPort()

	// Create pipe connections
	serverConn, clientConn := NewPipe()

	// Create a track on ServerPort for receiving uplink audio
	track, trackCtrl, err := serverPort.NewForegroundTrack()
	if err != nil {
		t.Fatalf("NewForegroundTrack: %v", err)
	}

	// Create decoder for the server side to decode uplink audio
	serverDecoder, err := opus.NewDecoder(sampleRate, format.Channels())
	if err != nil {
		t.Fatalf("NewDecoder: %v", err)
	}
	defer serverDecoder.Close()

	// Debug counters
	var uplinkFrames, decodedFrames, downlinkFrames, nonSilentFrames int
	var trackWritten int

	// Channel for signaling track data is ready
	trackDataReady := make(chan struct{})

	// Channel for downlink frames
	downlinkCh := make(chan StampedOpusFrame, 100)

	// 1. Client: Read from Mic -> Encode -> Send uplink
	go func() {
		clientPort.ReadFromMic(mic)
	}()

	go func() {
		clientPort.WriteTo(clientConn)
	}()

	// 2. Server: Read uplink from pipe
	go func() {
		serverPort.ReadFrom(serverConn)
	}()

	// 3. Server: Poll uplink data, decode audio, write to Track (Mixer)
	go func() {
		decodeBuf := make([]int16, frameSize*2)
		for {
			data, err := serverPort.Poll()
			if err != nil {
				// Queue closed - close track write to allow draining
				trackCtrl.CloseWrite()
				return
			}
			if data.Audio != nil {
				uplinkFrames++
				// Decode opus frame and write to track
				n, err := serverDecoder.DecodeTo(data.Audio.Frame, decodeBuf)
				if err != nil {
					continue // Skip invalid frames
				}
				decodedFrames++
				// Convert int16 to bytes and create chunk for track.Write
				pcmBytes := int16ToBytes(decodeBuf[:n])
				chunk := format.DataChunk(pcmBytes)
				if err := track.Write(chunk); err == nil {
					trackWritten++
					// Signal that all data is ready
					if trackWritten == numFrames {
						close(trackDataReady)
					}
				}
			}
		}
	}()

	// Wait for track data to be written before starting WriteTo
	// The mixer blocks on track.Read when track exists but has no data
	select {
	case <-trackDataReady:
		t.Logf("Track data ready: %d frames written", trackWritten)
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for track data")
	}

	// Close track write to signal end of data
	trackCtrl.CloseWrite()

	// 4. Server: Now start WriteTo (streamAudioTo reads from Mixer)
	go func() {
		serverPort.WriteTo(serverConn)
	}()

	// 5. Client: Read downlink from pipe
	go func() {
		for frame, err := range clientConn.OpusFrames() {
			if err != nil {
				close(downlinkCh)
				return
			}
			downlinkFrames++
			downlinkCh <- frame
		}
	}()

	// Also read commands
	go func() {
		for cmd, err := range clientConn.Commands() {
			if err != nil {
				return
			}
			_ = cmd
		}
	}()

	// 6. Collect downlink frames, filter silence, and decode to speaker
	decoder, err := opus.NewDecoder(sampleRate, format.Channels())
	if err != nil {
		t.Fatalf("NewDecoder for speaker: %v", err)
	}
	defer decoder.Close()

	decodeBuf := make([]int16, frameSize*2)
	timeout := time.After(2 * time.Second)
	expectedNonSilent := numFrames

collectLoop:
	for {
		select {
		case frame, ok := <-downlinkCh:
			if !ok {
				break collectLoop
			}
			// Decode frame
			n, err := decoder.DecodeTo(frame.Frame, decodeBuf)
			if err != nil {
				continue
			}

			// Check if frame is silence (all samples near zero)
			if !isSilence(decodeBuf[:n]) {
				nonSilentFrames++
				speaker.Write(decodeBuf[:n])
				if nonSilentFrames >= expectedNonSilent {
					break collectLoop
				}
			}
		case <-timeout:
			t.Log("Timeout waiting for downlink frames")
			break collectLoop
		}
	}

	// Close everything to stop goroutines
	clientPort.Close()
	serverPort.Close()
	clientConn.Close()
	serverConn.Close()

	// Give goroutines time to finish
	time.Sleep(100 * time.Millisecond)

	t.Logf("Uplink frames received: %d", uplinkFrames)
	t.Logf("Decoded frames: %d", decodedFrames)
	t.Logf("Track writes: %d", trackWritten)
	t.Logf("Downlink frames: %d (non-silent: %d)", downlinkFrames, nonSilentFrames)
	t.Logf("Input samples: %d, Output samples: %d", len(inputPCM), len(speaker.data))

	if len(speaker.data) == 0 {
		t.Fatal("No audio received at speaker - streamAudioTo may not be working")
	}

	// Verify correlation between input and output
	correlation := pcmCorrelation(inputPCM, speaker.data)
	t.Logf("Loopback correlation: %.4f", correlation)

	// For a full loopback (2x encode/decode), we expect lower correlation
	// due to cumulative codec artifacts, but should still be positive
	if correlation < 0.5 {
		t.Errorf("Loopback correlation too low: %.4f (expected > 0.5)", correlation)
	}
}

// isSilence checks if PCM samples are silence (all values near zero)
func isSilence(samples []int16) bool {
	const threshold int16 = 100 // Allow small noise
	for _, s := range samples {
		if s > threshold || s < -threshold {
			return false
		}
	}
	return true
}

// loopbackMic is a mock Mic for loopback testing
type loopbackMic struct {
	format    pcm.Format
	data      []int16
	frameSize int
	pos       int
}

func (m *loopbackMic) Read(buf []int16) (int, error) {
	if m.pos >= len(m.data) {
		return 0, io.EOF
	}

	n := copy(buf, m.data[m.pos:])
	m.pos += n
	return n, nil
}

func (m *loopbackMic) Format() pcm.Format {
	return m.format
}

// loopbackSpeaker is a mock Speaker for loopback testing
type loopbackSpeaker struct {
	format pcm.Format
	data   []int16
}

func (s *loopbackSpeaker) Write(buf []int16) (int, error) {
	s.data = append(s.data, buf...)
	return len(buf), nil
}

func (s *loopbackSpeaker) Format() pcm.Format {
	return s.format
}

// int16ToBytes converts int16 slice to bytes (for pcm.Track.Write)
func int16ToBytes(samples []int16) []byte {
	bytes := make([]byte, len(samples)*2)
	for i, s := range samples {
		bytes[i*2] = byte(s)
		bytes[i*2+1] = byte(s >> 8)
	}
	return bytes
}

// TestServerPort_MixerToDownlink tests if mixer data flows to downlink correctly
func TestServerPort_MixerToDownlink(t *testing.T) {
	format := pcm.L16Mono24K
	frameSize := int(format.SamplesInDuration(20 * time.Millisecond))

	// Generate test data
	testPCM := generateSineWave(frameSize*3, 440, format.SampleRate())

	// Create server port
	serverPort := NewServerPort()

	// Create pipe (we only use server side for downlink)
	serverConn, clientConn := NewPipe()

	// Create track and write data to it
	track, trackCtrl, err := serverPort.NewForegroundTrack()
	if err != nil {
		t.Fatalf("NewForegroundTrack: %v", err)
	}

	// Write PCM data to track
	pcmBytes := int16ToBytes(testPCM)
	chunk := format.DataChunk(pcmBytes)
	if err := track.Write(chunk); err != nil {
		t.Fatalf("track.Write: %v", err)
	}
	t.Logf("Wrote %d bytes to track", len(pcmBytes))

	// Close track to signal end of data
	trackCtrl.CloseWithError(nil)

	// Count downlink frames
	var downlinkFrames int
	frameCh := make(chan StampedOpusFrame, 100)

	// Read downlink from client side
	go func() {
		for frame, err := range clientConn.OpusFrames() {
			if err != nil {
				return
			}
			frameCh <- frame
		}
	}()

	// Start WriteTo (which calls streamAudioTo)
	writeDone := make(chan error, 1)
	go func() {
		writeDone <- serverPort.WriteTo(serverConn)
	}()

	// Wait for some frames
	timeout := time.After(2 * time.Second)
	for {
		select {
		case <-frameCh:
			downlinkFrames++
			if downlinkFrames >= 3 {
				goto done
			}
		case <-timeout:
			goto done
		}
	}

done:
	// Cleanup
	serverPort.Close()
	serverConn.Close()
	clientConn.Close()

	// Drain writeDone
	select {
	case <-writeDone:
	case <-time.After(100 * time.Millisecond):
	}

	t.Logf("Downlink frames received: %d", downlinkFrames)

	if downlinkFrames == 0 {
		t.Error("No downlink frames received - streamAudioTo may not be working")
	}
}

// TestMixer_DirectReadWrite tests mixer read/write directly
func TestMixer_DirectReadWrite(t *testing.T) {
	format := pcm.L16Mono24K
	frameSize := int(format.SamplesInDuration(20 * time.Millisecond))

	// Create mixer directly
	mixer := pcm.NewMixer(format)

	// Create track
	track, trackCtrl, err := mixer.CreateTrack()
	if err != nil {
		t.Fatalf("CreateTrack: %v", err)
	}

	// Generate test data
	testPCM := generateSineWave(frameSize*3, 440, format.SampleRate())
	pcmBytes := int16ToBytes(testPCM)
	chunk := format.DataChunk(pcmBytes)

	// Write to track
	if err := track.Write(chunk); err != nil {
		t.Fatalf("track.Write: %v", err)
	}
	t.Logf("Wrote %d bytes to track", len(pcmBytes))

	// CloseWrite on track signals end of data for this track
	// but allows the track to be drained (unlike CloseWithError)
	trackCtrl.CloseWrite()

	// Close mixer writing to signal no more tracks will be added
	// This allows mixer.Read to return EOF after all tracks are drained
	mixer.CloseWrite()

	// Read from mixer
	readBuf := make([]byte, frameSize*2)
	var totalRead int
	for i := 0; i < 5; i++ {
		n, err := mixer.Read(readBuf)
		if err != nil {
			t.Logf("Read[%d] error: %v", i, err)
			break
		}
		totalRead += n
		t.Logf("Read[%d]: %d bytes", i, n)
	}

	t.Logf("Total read: %d bytes", totalRead)
	if totalRead == 0 {
		t.Error("No data read from mixer")
	}
}

// TestMixer_ConcurrentReadWrite tests mixer with concurrent read/write
func TestMixer_ConcurrentReadWrite(t *testing.T) {
	format := pcm.L16Mono24K
	frameSize := int(format.SamplesInDuration(20 * time.Millisecond))

	// Create mixer with silence gap so it can output silence when no data
	mixer := pcm.NewMixer(format, pcm.WithSilenceGap(10*time.Second))

	// Create track
	track, trackCtrl, err := mixer.CreateTrack()
	if err != nil {
		t.Fatalf("CreateTrack: %v", err)
	}

	// Generate and write test data FIRST
	testPCM := generateSineWave(frameSize*3, 440, format.SampleRate())
	pcmBytes := int16ToBytes(testPCM)
	chunk := format.DataChunk(pcmBytes)
	if err := track.Write(chunk); err != nil {
		t.Fatalf("track.Write: %v", err)
	}
	t.Logf("Wrote %d bytes to track", len(pcmBytes))

	// Read from mixer concurrently
	readBuf := make([]byte, frameSize*2)
	readCh := make(chan int, 10)
	go func() {
		for {
			n, err := mixer.Read(readBuf)
			if err != nil {
				close(readCh)
				return
			}
			readCh <- n
		}
	}()

	// Wait for some reads
	time.Sleep(100 * time.Millisecond)

	// Close track and mixer to allow reader to exit
	trackCtrl.CloseWrite()
	mixer.CloseWrite()

	// Count reads
	var totalRead int
	for n := range readCh {
		totalRead += n
	}

	t.Logf("Total read: %d bytes", totalRead)
	if totalRead == 0 {
		t.Error("No data read from mixer")
	}
}
