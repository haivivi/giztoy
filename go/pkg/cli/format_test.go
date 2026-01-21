package cli

import "testing"

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		ms   int
		want string
	}{
		{0, "0ms"},
		{1, "1ms"},
		{100, "100ms"},
		{999, "999ms"},
		{1000, "1.0s"},
		{1500, "1.5s"},
		{5000, "5.0s"},
		{59000, "59.0s"},
		{60000, "1m0.0s"},
		{61000, "1m1.0s"},
		{90000, "1m30.0s"},
		{120000, "2m0.0s"},
		{125500, "2m5.5s"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := FormatDuration(tt.ms)
			if got != tt.want {
				t.Errorf("FormatDuration(%d) = %q, want %q", tt.ms, got, tt.want)
			}
		})
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		bytes int64
		want  string
	}{
		{0, "0 B"},
		{1, "1 B"},
		{100, "100 B"},
		{1023, "1023 B"},
		{1024, "1.00 KB"},
		{1536, "1.50 KB"},
		{10240, "10.00 KB"},
		{1048576, "1.00 MB"},
		{1572864, "1.50 MB"},
		{10485760, "10.00 MB"},
		{1073741824, "1.00 GB"},
		{1610612736, "1.50 GB"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := FormatBytes(tt.bytes)
			if got != tt.want {
				t.Errorf("FormatBytes(%d) = %q, want %q", tt.bytes, got, tt.want)
			}
		})
	}
}

func TestFormatBytesInt(t *testing.T) {
	tests := []struct {
		bytes int
		want  string
	}{
		{0, "0 B"},
		{1024, "1.00 KB"},
		{1048576, "1.00 MB"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := FormatBytesInt(tt.bytes)
			if got != tt.want {
				t.Errorf("FormatBytesInt(%d) = %q, want %q", tt.bytes, got, tt.want)
			}
		})
	}
}
