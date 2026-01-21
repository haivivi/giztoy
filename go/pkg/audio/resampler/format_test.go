package resampler

import "testing"

func TestFormat_channels(t *testing.T) {
	tests := []struct {
		name   string
		format Format
		want   int
	}{
		{
			name:   "mono",
			format: Format{SampleRate: 44100, Stereo: false},
			want:   1,
		},
		{
			name:   "stereo",
			format: Format{SampleRate: 48000, Stereo: true},
			want:   2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.format.channels(); got != tt.want {
				t.Errorf("Format.channels() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestFormat_sampleBytes(t *testing.T) {
	tests := []struct {
		name   string
		format Format
		want   int
	}{
		{
			name:   "mono 16-bit",
			format: Format{SampleRate: 44100, Stereo: false},
			want:   2, // 1 channel * 2 bytes (16-bit)
		},
		{
			name:   "stereo 16-bit",
			format: Format{SampleRate: 48000, Stereo: true},
			want:   4, // 2 channels * 2 bytes (16-bit)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.format.sampleBytes(); got != tt.want {
				t.Errorf("Format.sampleBytes() = %d, want %d", got, tt.want)
			}
		})
	}
}

