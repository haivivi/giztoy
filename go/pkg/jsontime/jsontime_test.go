package jsontime

import (
	"encoding/json"
	"testing"
	"time"
)

func TestMilli_MarshalJSON(t *testing.T) {
	// Test specific time
	tm := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	ep := Milli(tm)

	data, err := json.Marshal(ep)
	if err != nil {
		t.Fatalf("MarshalJSON error: %v", err)
	}

	expected := tm.UnixMilli()
	var got int64
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal result error: %v", err)
	}
	if got != expected {
		t.Errorf("MarshalJSON = %d, want %d", got, expected)
	}
}

func TestMilli_UnmarshalJSON(t *testing.T) {
	ms := int64(1705315800000) // 2024-01-15 10:30:00 UTC
	data, _ := json.Marshal(ms)

	var ep Milli
	if err := json.Unmarshal(data, &ep); err != nil {
		t.Fatalf("UnmarshalJSON error: %v", err)
	}

	expected := time.UnixMilli(ms)
	if !time.Time(ep).Equal(expected) {
		t.Errorf("UnmarshalJSON = %v, want %v", time.Time(ep), expected)
	}
}

func TestMilli_RoundTrip(t *testing.T) {
	original := NowEpochMilli()

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var restored Milli
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	// Milli precision: compare at millisecond level
	if original.Time().UnixMilli() != restored.Time().UnixMilli() {
		t.Errorf("RoundTrip: original=%v, restored=%v", original, restored)
	}
}

func TestMilli_Comparisons(t *testing.T) {
	t1 := Milli(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))
	t2 := Milli(time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC))

	if !t1.Before(t2) {
		t.Error("t1 should be before t2")
	}
	if !t2.After(t1) {
		t.Error("t2 should be after t1")
	}
	if t1.Equal(t2) {
		t.Error("t1 should not equal t2")
	}
	if !t1.Equal(t1) {
		t.Error("t1 should equal itself")
	}
}

func TestMilli_Methods(t *testing.T) {
	ep := NowEpochMilli()

	// Test String
	if ep.String() == "" {
		t.Error("String() should not be empty")
	}

	// Test Time
	if ep.Time().IsZero() {
		t.Error("Time() should not be zero")
	}

	// Test IsZero
	var zero Milli
	if !zero.IsZero() {
		t.Error("zero Milli should be zero")
	}

	// Test Add/Sub
	added := ep.Add(time.Hour)
	if added.Sub(ep) != time.Hour {
		t.Error("Add/Sub should work correctly")
	}
}

func TestUnix_MarshalJSON(t *testing.T) {
	tm := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	ep := Unix(tm)

	data, err := json.Marshal(ep)
	if err != nil {
		t.Fatalf("MarshalJSON error: %v", err)
	}

	expected := tm.Unix()
	var got int64
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal result error: %v", err)
	}
	if got != expected {
		t.Errorf("MarshalJSON = %d, want %d", got, expected)
	}
}

func TestUnix_UnmarshalJSON(t *testing.T) {
	sec := int64(1705315800) // 2024-01-15 10:30:00 UTC
	data, _ := json.Marshal(sec)

	var ep Unix
	if err := json.Unmarshal(data, &ep); err != nil {
		t.Fatalf("UnmarshalJSON error: %v", err)
	}

	expected := time.Unix(sec, 0)
	if !time.Time(ep).Equal(expected) {
		t.Errorf("UnmarshalJSON = %v, want %v", time.Time(ep), expected)
	}
}

func TestUnix_RoundTrip(t *testing.T) {
	original := NowEpoch()

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var restored Unix
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	// Unix precision: compare at second level
	if original.Time().Unix() != restored.Time().Unix() {
		t.Errorf("RoundTrip: original=%v, restored=%v", original, restored)
	}
}

func TestUnix_Comparisons(t *testing.T) {
	t1 := Unix(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))
	t2 := Unix(time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC))

	if !t1.Before(t2) {
		t.Error("t1 should be before t2")
	}
	if !t2.After(t1) {
		t.Error("t2 should be after t1")
	}
	if t1.Equal(t2) {
		t.Error("t1 should not equal t2")
	}
}

func TestUnix_Methods(t *testing.T) {
	ep := NowEpoch()

	// Test String
	if ep.String() == "" {
		t.Error("String() should not be empty")
	}

	// Test Time
	if ep.Time().IsZero() {
		t.Error("Time() should not be zero")
	}

	// Test IsZero
	var zero Unix
	if !zero.IsZero() {
		t.Error("zero Unix should be zero")
	}

	// Test Add/Sub
	added := ep.Add(time.Hour)
	if added.Sub(ep) != time.Hour {
		t.Error("Add/Sub should work correctly")
	}
}

func TestDuration_MarshalJSON(t *testing.T) {
	d := Duration(90 * time.Minute)

	data, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("MarshalJSON error: %v", err)
	}

	var got string
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal result error: %v", err)
	}
	if got != "1h30m0s" {
		t.Errorf("MarshalJSON = %q, want %q", got, "1h30m0s")
	}
}

func TestDuration_UnmarshalJSON_String(t *testing.T) {
	data := []byte(`"2h30m"`)

	var d Duration
	if err := json.Unmarshal(data, &d); err != nil {
		t.Fatalf("UnmarshalJSON error: %v", err)
	}

	expected := 2*time.Hour + 30*time.Minute
	if time.Duration(d) != expected {
		t.Errorf("UnmarshalJSON = %v, want %v", time.Duration(d), expected)
	}
}

func TestDuration_UnmarshalJSON_Int(t *testing.T) {
	ns := int64(5 * time.Second)
	data, _ := json.Marshal(ns)

	var d Duration
	if err := json.Unmarshal(data, &d); err != nil {
		t.Fatalf("UnmarshalJSON error: %v", err)
	}

	if time.Duration(d) != 5*time.Second {
		t.Errorf("UnmarshalJSON = %v, want %v", time.Duration(d), 5*time.Second)
	}
}

func TestDuration_UnmarshalJSON_Null(t *testing.T) {
	data := []byte(`null`)

	var d Duration
	if err := json.Unmarshal(data, &d); err != nil {
		t.Fatalf("UnmarshalJSON error: %v", err)
	}

	if time.Duration(d) != 0 {
		t.Errorf("UnmarshalJSON null = %v, want 0", time.Duration(d))
	}
}

func TestDuration_RoundTrip(t *testing.T) {
	original := Duration(3*time.Hour + 45*time.Minute + 30*time.Second)

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var restored Duration
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if original != restored {
		t.Errorf("RoundTrip: original=%v, restored=%v", original, restored)
	}
}

func TestDuration_Methods(t *testing.T) {
	d := Duration(90 * time.Minute)

	// Test Duration()
	if d.Duration() != 90*time.Minute {
		t.Errorf("Duration() = %v, want %v", d.Duration(), 90*time.Minute)
	}

	// Test nil Duration()
	var nilD *Duration
	if nilD.Duration() != 0 {
		t.Error("nil Duration() should return 0")
	}

	// Test String()
	if d.String() != "1h30m0s" {
		t.Errorf("String() = %q, want %q", d.String(), "1h30m0s")
	}

	// Test Seconds()
	if d.Seconds() != 5400 {
		t.Errorf("Seconds() = %v, want 5400", d.Seconds())
	}

	// Test Milliseconds()
	if d.Milliseconds() != 5400000 {
		t.Errorf("Milliseconds() = %v, want 5400000", d.Milliseconds())
	}
}

func TestFromDuration(t *testing.T) {
	d := time.Hour
	ptr := FromDuration(d)

	if ptr == nil {
		t.Fatal("FromDuration returned nil")
	}
	if *ptr != Duration(d) {
		t.Errorf("FromDuration = %v, want %v", *ptr, Duration(d))
	}
}

func TestDuration_InStruct(t *testing.T) {
	type Config struct {
		Timeout  Duration  `json:"timeout"`
		Interval *Duration `json:"interval,omitempty"`
	}

	// Test with values
	cfg := Config{
		Timeout:  Duration(30 * time.Second),
		Interval: FromDuration(5 * time.Minute),
	}

	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var restored Config
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if restored.Timeout != cfg.Timeout {
		t.Errorf("Timeout = %v, want %v", restored.Timeout, cfg.Timeout)
	}
	if restored.Interval.Duration() != cfg.Interval.Duration() {
		t.Errorf("Interval = %v, want %v", restored.Interval.Duration(), cfg.Interval.Duration())
	}
}
