package songs

import (
	"encoding/binary"
	"io"
	"math"
	"sync"

	"github.com/haivivi/giztoy/pkg/audio/pcm"
)

// DefaultFormat is the default audio format for songs.
const DefaultFormat = pcm.L16Mono16K

// GenerateSineWave generates a pure sine wave as 16-bit PCM data (little-endian).
func GenerateSineWave(freq float64, samples int, sampleRate int) []byte {
	data := make([]byte, samples*2)
	if freq == Rest {
		return data
	}
	for i := range samples {
		t := float64(i) / float64(sampleRate)
		value := math.Sin(2 * math.Pi * freq * t)
		sample := int16(value * 16000)
		binary.LittleEndian.PutUint16(data[i*2:], uint16(sample))
	}
	return data
}

// GenerateRichNote generates a piano-like note with realistic harmonics and envelope.
func GenerateRichNote(freq float64, samples int, sampleRate int, volume float64) []int16 {
	data := make([]int16, samples)
	if freq == Rest {
		return data
	}

	// Piano harmonic structure (relative amplitudes)
	// Real piano has complex inharmonicity - higher partials are slightly sharp
	harmonics := []struct {
		ratio     float64 // frequency ratio
		amplitude float64 // relative amplitude
		decay     float64 // decay rate multiplier (higher = faster decay)
	}{
		{1.0, 1.0, 1.0},   // fundamental
		{2.0, 0.7, 1.2},   // 2nd harmonic
		{3.0, 0.45, 1.5},  // 3rd harmonic
		{4.0, 0.3, 1.8},   // 4th harmonic
		{5.0, 0.2, 2.2},   // 5th harmonic
		{6.0, 0.12, 2.6},  // 6th harmonic
		{7.0, 0.08, 3.0},  // 7th harmonic
		{8.0, 0.05, 3.5},  // 8th harmonic
		{9.0, 0.03, 4.0},  // 9th harmonic
		{10.0, 0.02, 4.5}, // 10th harmonic
	}

	// Piano inharmonicity coefficient (strings are slightly stiff)
	// Higher notes have more inharmonicity
	inharmonicity := 0.0001 * (freq / 440.0) * (freq / 440.0)

	// Note duration for decay calculation
	noteDuration := float64(samples) / float64(sampleRate)

	for i := 0; i < samples; i++ {
		t := float64(i) / float64(sampleRate)
		progress := t / noteDuration

		var sample float64

		for _, h := range harmonics {
			// Apply inharmonicity: higher harmonics are slightly sharp
			actualRatio := h.ratio * math.Sqrt(1.0+inharmonicity*h.ratio*h.ratio)
			phase := 2 * math.Pi * freq * actualRatio * t

			// Harmonic amplitude with time-dependent decay
			// Higher harmonics decay faster (more realistic piano sound)
			harmonicDecay := math.Exp(-progress * h.decay * 3.0)
			amplitude := h.amplitude * harmonicDecay

			sample += amplitude * math.Sin(phase)
		}

		// Normalize the harmonic sum
		sample /= 2.5

		// Piano envelope: fast attack, gradual decay
		envelope := computePianoEnvelope(i, samples, sampleRate, noteDuration)

		// Apply volume and convert to int16
		sample *= volume * envelope
		data[i] = int16(clamp(sample, -1.0, 1.0) * 32767 * 0.85)
	}

	return data
}

// computePianoEnvelope generates a piano-like ADSR envelope.
func computePianoEnvelope(i, samples, sampleRate int, noteDuration float64) float64 {
	t := float64(i) / float64(sampleRate)
	progress := float64(i) / float64(samples)

	// Fast attack (2-5ms for piano hammer strike)
	attackTime := 0.003 // 3ms

	// Decay depends on note length
	// Shorter notes have more prominent decay
	decayRate := 2.0 / noteDuration
	if decayRate < 0.5 {
		decayRate = 0.5
	}
	if decayRate > 8.0 {
		decayRate = 8.0
	}

	// Release at the end (soft pedal release effect)
	releaseStart := 0.85

	if t < attackTime {
		// Attack phase: exponential rise for percussive feel
		attackProgress := t / attackTime
		return 1.0 - math.Exp(-5.0*attackProgress)
	} else if progress < releaseStart {
		// Decay/Sustain phase: exponential decay
		decayT := t - attackTime
		return math.Exp(-decayT*decayRate)*0.95 + 0.05
	} else {
		// Release phase
		releaseProgress := (progress - releaseStart) / (1.0 - releaseStart)
		baseLevel := math.Exp(-(t-attackTime)*decayRate)*0.95 + 0.05
		return baseLevel * (1.0 - releaseProgress*releaseProgress)
	}
}

// GenerateChord generates a chord with piano-like sound.
func GenerateChord(freqs []float64, samples int, sampleRate int, volume float64) []int16 {
	data := make([]int16, samples)
	if len(freqs) == 0 {
		return data
	}

	// Filter out rests
	activeFreqs := make([]float64, 0, len(freqs))
	for _, f := range freqs {
		if f != Rest {
			activeFreqs = append(activeFreqs, f)
		}
	}
	if len(activeFreqs) == 0 {
		return data
	}

	// Generate each note separately and mix
	voiceVolume := volume / math.Sqrt(float64(len(activeFreqs)))

	for _, freq := range activeFreqs {
		noteData := GenerateRichNote(freq, samples, sampleRate, voiceVolume)
		for i := range data {
			// Mix with soft clipping
			mixed := float64(data[i]) + float64(noteData[i])
			data[i] = int16(clamp(mixed/32767.0, -1.0, 1.0) * 32767)
		}
	}

	return data
}

// clamp restricts a value to the given range.
func clamp(value, min, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

// Int16ToBytes converts []int16 samples to raw PCM bytes (little-endian).
func Int16ToBytes(samples []int16) []byte {
	data := make([]byte, len(samples)*2)
	for i, s := range samples {
		binary.LittleEndian.PutUint16(data[i*2:], uint16(s))
	}
	return data
}

// DurationSamples calculates the number of samples for a given duration in ms.
func DurationSamples(durMs int, sampleRate int) int {
	return sampleRate * durMs / 1000
}

// DurationSamplesFormat calculates samples for a duration using pcm.Format.
func DurationSamplesFormat(durMs int, format pcm.Format) int {
	return format.SampleRate() * durMs / 1000
}

// TotalDuration returns the total duration of a melody in milliseconds.
func TotalDuration(melody []Note) int {
	total := 0
	for _, n := range melody {
		total += n.Dur
	}
	return total
}

// RenderOptions configures song rendering.
type RenderOptions struct {
	Format    pcm.Format // Audio format (default: L16Mono16K)
	Volume    float64    // Volume 0.0-1.0 (default: 0.5)
	Metronome bool       // Include metronome track
	RichSound bool       // Use piano-like rich harmonics (default: true)
}

// DefaultRenderOptions returns default rendering options.
func DefaultRenderOptions() RenderOptions {
	return RenderOptions{
		Format:    DefaultFormat,
		Volume:    0.5,
		Metronome: false,
		RichSound: true,
	}
}

// Render renders a song to mixed PCM audio using pcm.Mixer.
// It creates a track for each voice and mixes them together.
// Returns an io.Reader that reads the mixed audio data.
func (s Song) Render(opts RenderOptions) io.Reader {
	if opts.Format == 0 {
		opts.Format = DefaultFormat
	}
	if opts.Volume == 0 {
		opts.Volume = 0.5
	}

	voices := s.ToVoices(opts.Metronome)
	if len(voices) == 0 {
		return &emptyReader{}
	}

	// Create mixer
	mixer := pcm.NewMixer(opts.Format, pcm.WithAutoClose())

	// Pre-generate all voice data to ensure proper mixing
	// This avoids timing issues where mixer reads before all tracks have data
	voiceChunks := make([]pcm.Chunk, len(voices))
	for i, voice := range voices {
		voiceChunks[i] = VoiceToChunk(voice, opts.Format, opts.Volume)
	}

	// Create tracks and write all data
	var wg sync.WaitGroup
	for i, chunk := range voiceChunks {
		track, ctrl, err := mixer.CreateTrack(pcm.WithTrackLabel(voiceLabel(i)))
		if err != nil {
			continue
		}

		wg.Add(1)
		go func(track pcm.Track, ctrl *pcm.TrackCtrl, chunk pcm.Chunk) {
			defer wg.Done()
			defer ctrl.CloseWrite()
			track.Write(chunk)
		}(track, ctrl, chunk)
	}

	// Close mixer when all writers are done
	go func() {
		wg.Wait()
		mixer.CloseWrite()
	}()

	return mixer
}

// RenderBytes renders a song to a byte slice.
func (s Song) RenderBytes(opts RenderOptions) ([]byte, error) {
	r := s.Render(opts)
	return io.ReadAll(r)
}

// voiceLabel returns a label for a voice index.
func voiceLabel(i int) string {
	switch i {
	case 0:
		return "melody"
	case 1:
		return "bass"
	case 2:
		return "harmony"
	default:
		return "voice"
	}
}

// emptyReader is an io.Reader that always returns EOF.
type emptyReader struct{}

func (emptyReader) Read(p []byte) (int, error) {
	return 0, io.EOF
}

// VoiceToChunk converts a Voice to a pcm.Chunk.
func VoiceToChunk(voice Voice, format pcm.Format, volume float64) pcm.Chunk {
	sampleRate := format.SampleRate()
	var totalSamples int
	for _, n := range voice.Notes {
		totalSamples += DurationSamples(n.Dur, sampleRate)
	}

	data := make([]byte, totalSamples*2)
	offset := 0

	for _, n := range voice.Notes {
		samples := DurationSamples(n.Dur, sampleRate)
		noteData := GenerateRichNote(n.Freq, samples, sampleRate, volume)

		for _, s := range noteData {
			binary.LittleEndian.PutUint16(data[offset:], uint16(s))
			offset += 2
		}
	}

	return format.DataChunk(data)
}
