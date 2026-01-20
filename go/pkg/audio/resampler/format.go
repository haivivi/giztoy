package resampler

// Format describes the audio format for resampling. Currently only supports
// 16-bit signed integer samples.
type Format struct {
	// SampleRate is the sample rate in Hz (e.g., 44100, 48000).
	SampleRate int

	// Stereo indicates stereo (2 channels) if true, mono (1 channel) if false.
	Stereo bool
}

func (f Format) channels() int {
	if f.Stereo {
		return 2
	}
	return 1
}

func (f Format) sampleBytes() int {
	if f.Stereo {
		return 4
	}
	return 2
}
