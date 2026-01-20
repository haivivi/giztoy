package songs

import (
	"testing"
)

func TestAll(t *testing.T) {
	if len(All) == 0 {
		t.Error("All songs list is empty")
	}

	// Verify each song has valid data
	for _, s := range All {
		if s.ID == "" {
			t.Error("song ID is empty")
		}
		if s.Name == "" {
			t.Error("song name is empty")
		}

		voices := s.ToVoices(false)
		if len(voices) == 0 {
			t.Errorf("song %s has no voices", s.ID)
		}
		for i, v := range voices {
			if len(v.Notes) == 0 {
				t.Errorf("song %s voice %d has no notes", s.ID, i)
			}
		}
	}
}

func TestByID(t *testing.T) {
	tests := []struct {
		id   string
		want string
	}{
		{"happy_birthday", "生日快乐"},
		{"two_tigers", "两只老虎"},
		{"doll_and_bear", "洋娃娃和小熊跳舞"},
		{"fur_elise", "献给爱丽丝"},
		{"twinkle_star", "小星星"},
		{"canon", "卡农"},
		{"nonexistent", ""},
	}

	for _, tt := range tests {
		s := ByID(tt.id)
		if tt.want == "" {
			if s != nil {
				t.Errorf("ByID(%q) expected nil, got %v", tt.id, s)
			}
		} else {
			if s == nil {
				t.Errorf("ByID(%q) returned nil", tt.id)
			} else if s.Name != tt.want {
				t.Errorf("ByID(%q).Name = %q, want %q", tt.id, s.Name, tt.want)
			}
		}
	}
}

func TestByName(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"生日快乐", "happy_birthday"},
		{"两只老虎", "two_tigers"},
		{"小星星", "twinkle_star"},
		{"nonexistent", ""},
	}

	for _, tt := range tests {
		s := ByName(tt.name)
		if tt.want == "" {
			if s != nil {
				t.Errorf("ByName(%q) expected nil, got %v", tt.name, s)
			}
		} else {
			if s == nil {
				t.Errorf("ByName(%q) returned nil", tt.name)
			} else if s.ID != tt.want {
				t.Errorf("ByName(%q).ID = %q, want %q", tt.name, s.ID, tt.want)
			}
		}
	}
}

func TestIDs(t *testing.T) {
	ids := IDs()
	if len(ids) != len(All) {
		t.Errorf("IDs() returned %d items, expected %d", len(ids), len(All))
	}

	// Check first few IDs
	expectedFirst := []string{"twinkle_star", "happy_birthday", "two_tigers", "doll_and_bear"}
	for i, expected := range expectedFirst {
		if i >= len(ids) {
			break
		}
		if ids[i] != expected {
			t.Errorf("IDs()[%d] = %q, want %q", i, ids[i], expected)
		}
	}
}

func TestNames(t *testing.T) {
	names := Names()
	if len(names) != len(All) {
		t.Errorf("Names() returned %d items, expected %d", len(names), len(All))
	}

	// Check first few names
	expectedFirst := []string{"小星星", "生日快乐", "两只老虎", "洋娃娃和小熊跳舞"}
	for i, expected := range expectedFirst {
		if i >= len(names) {
			break
		}
		if names[i] != expected {
			t.Errorf("Names()[%d] = %q, want %q", i, names[i], expected)
		}
	}
}

func TestSongDuration(t *testing.T) {
	dur := SongHappyBirthday.Duration()
	if dur <= 0 {
		t.Errorf("SongHappyBirthday.Duration() = %d, expected > 0", dur)
	}
}

func TestSongToVoices(t *testing.T) {
	voices := SongHappyBirthday.ToVoices(false)
	if len(voices) == 0 {
		t.Error("SongHappyBirthday.ToVoices(false) returned empty")
	}

	// With metronome
	voicesWithMetronome := SongHappyBirthday.ToVoices(true)
	if len(voicesWithMetronome) <= len(voices) {
		t.Error("ToVoices(true) should have more voices than ToVoices(false)")
	}
}

func TestGenerateSineWave(t *testing.T) {
	sampleRate := 16000
	durMs := 100
	samples := DurationSamples(durMs, sampleRate)

	// Test normal frequency
	data := GenerateSineWave(A4, samples, sampleRate)
	if len(data) != samples*2 {
		t.Errorf("GenerateSineWave length = %d, expected %d", len(data), samples*2)
	}

	// Verify non-zero values
	hasNonZero := false
	for _, b := range data {
		if b != 0 {
			hasNonZero = true
			break
		}
	}
	if !hasNonZero {
		t.Error("GenerateSineWave returned all zeros for A4")
	}

	// Test rest (silence)
	silenceData := GenerateSineWave(Rest, samples, sampleRate)
	for _, b := range silenceData {
		if b != 0 {
			t.Error("GenerateSineWave(Rest) should return all zeros")
			break
		}
	}
}

func TestGenerateRichNote(t *testing.T) {
	sampleRate := 16000
	durMs := 100
	samples := DurationSamples(durMs, sampleRate)

	// Test normal frequency
	data := GenerateRichNote(A4, samples, sampleRate, 0.5)
	if len(data) != samples {
		t.Errorf("GenerateRichNote length = %d, expected %d", len(data), samples)
	}

	// Verify non-zero values
	hasNonZero := false
	for _, s := range data {
		if s != 0 {
			hasNonZero = true
			break
		}
	}
	if !hasNonZero {
		t.Error("GenerateRichNote returned all zeros for A4")
	}

	// Test rest (silence)
	silenceData := GenerateRichNote(Rest, samples, sampleRate, 0.5)
	for _, s := range silenceData {
		if s != 0 {
			t.Error("GenerateRichNote(Rest) should return all zeros")
			break
		}
	}
}

func TestInt16ToBytes(t *testing.T) {
	samples := []int16{0x0102, 0x0304}
	data := Int16ToBytes(samples)

	expected := []byte{0x02, 0x01, 0x04, 0x03} // little-endian
	if len(data) != len(expected) {
		t.Errorf("Int16ToBytes length = %d, expected %d", len(data), len(expected))
	}
	for i, b := range data {
		if b != expected[i] {
			t.Errorf("Int16ToBytes[%d] = %02x, expected %02x", i, b, expected[i])
		}
	}
}

func TestDurationSamples(t *testing.T) {
	tests := []struct {
		durMs      int
		sampleRate int
		want       int
	}{
		{1000, 16000, 16000},
		{500, 16000, 8000},
		{100, 24000, 2400},
	}

	for _, tt := range tests {
		got := DurationSamples(tt.durMs, tt.sampleRate)
		if got != tt.want {
			t.Errorf("DurationSamples(%d, %d) = %d, want %d", tt.durMs, tt.sampleRate, got, tt.want)
		}
	}
}

func TestTempo(t *testing.T) {
	tempo := Tempo{BPM: 120, Signature: Time4_4}

	// At 120 BPM, one beat = 500ms
	beatDur := tempo.BeatDuration(1)
	if beatDur != 500 {
		t.Errorf("Tempo.BeatDuration(1) = %d, want 500", beatDur)
	}

	// One bar = 4 beats = 2000ms
	barDur := tempo.BarDuration()
	if barDur != 2000 {
		t.Errorf("Tempo.BarDuration() = %d, want 2000", barDur)
	}
}

func TestBeatNote(t *testing.T) {
	tempo := Tempo{BPM: 120, Signature: Time4_4}
	bn := N(A4, 1) // One beat at A4

	note := bn.ToNote(tempo)
	if note.Freq != A4 {
		t.Errorf("BeatNote.ToNote().Freq = %v, want %v", note.Freq, A4)
	}
	if note.Dur != 500 {
		t.Errorf("BeatNote.ToNote().Dur = %d, want 500", note.Dur)
	}
}
