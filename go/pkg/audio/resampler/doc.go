// Package resampler provides audio resampling using the SoX Resampler (soxr)
// library.
//
// It supports:
//   - Sample rate conversion (e.g., 44100Hz to 48000Hz)
//   - Channel conversion (mono to stereo or stereo to mono)
//   - Streaming interface via io.Reader
//
// The package uses high-quality resampling by default and handles 16-bit signed
// integer audio samples.
//
// Example usage:
//
//	src := resampler.Format{SampleRate: 44100, Stereo: true}
//	dst := resampler.Format{SampleRate: 48000, Stereo: false}
//	r, err := resampler.New(audioReader, src, dst)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	// Read resampled audio from r
//	io.Copy(output, r)
package resampler
