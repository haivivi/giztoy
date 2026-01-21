package speech

import (
	"strings"
	"testing"

	"google.golang.org/api/iterator"
)

func TestDefaultSentenceSegmenter(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "simple sentences",
			input:    "Hello world。This is a test。",
			expected: []string{"Hello world。", "This is a test。"},
		},
		{
			name:     "multiple punctuation",
			input:    "First sentence。Second sentence！Third？",
			expected: []string{"First sentence。", "Second sentence！Third？"},
		},
		{
			name:     "comma splitting",
			input:    "Part one，part two，part three。",
			expected: []string{"Part one，", "part two，part three。"},
		},
		{
			name:     "number with decimal",
			input:    "The price is 9.9 dollars。",
			expected: []string{"The price is 9.9 dollars。"},
		},
		{
			name:     "time format",
			input:    "Meeting at 10:30 today。",
			expected: []string{"Meeting at 10:30 today。"},
		},
		{
			name:     "newline",
			input:    "Line one\nLine two",
			expected: []string{"Line one\n", "Line two"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			segmenter := DefaultSentenceSegmenter{}
			iter, err := segmenter.Segment(strings.NewReader(tc.input))
			if err != nil {
				t.Fatalf("Segment error: %v", err)
			}
			defer iter.Close()

			var results []string
			for {
				seg, err := iter.Next()
				if err != nil {
					if err == iterator.Done {
						break
					}
					t.Fatalf("Next error: %v", err)
				}
				results = append(results, seg)
			}

			if len(results) != len(tc.expected) {
				t.Errorf("got %d segments; want %d", len(results), len(tc.expected))
				t.Logf("results: %v", results)
				t.Logf("expected: %v", tc.expected)
				return
			}

			for i, seg := range results {
				if seg != tc.expected[i] {
					t.Errorf("segment[%d] = %q; want %q", i, seg, tc.expected[i])
				}
			}
		})
	}
}

func TestDefaultSentenceSegmenter_MaxRunes(t *testing.T) {
	segmenter := DefaultSentenceSegmenter{MaxRunesPerSegment: 10}

	// Text longer than 10 runes without punctuation
	input := "abcdefghijklmnopqrstuvwxyz"
	iter, err := segmenter.Segment(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Segment error: %v", err)
	}
	defer iter.Close()

	var results []string
	for {
		seg, err := iter.Next()
		if err != nil {
			if err == iterator.Done {
				break
			}
			t.Fatalf("Next error: %v", err)
		}
		results = append(results, seg)
	}

	// Should be split into multiple segments
	if len(results) < 2 {
		t.Errorf("expected multiple segments for long text, got %d", len(results))
	}

	// Each segment should be at most 10 runes
	for i, seg := range results {
		if len([]rune(seg)) > 10 {
			t.Errorf("segment[%d] = %d runes; want <= 10", i, len([]rune(seg)))
		}
	}
}

func TestLastRuneIndex(t *testing.T) {
	tests := []struct {
		input    []byte
		expected int
	}{
		{[]byte("hello"), 5},
		{[]byte("你好"), 6},               // 2 Chinese chars, 3 bytes each
		{[]byte("hello世界"), 11},         // 5 ASCII + 2 Chinese
		{[]byte{0xE4, 0xB8}, 0},          // Incomplete UTF-8
		{[]byte{0xE4, 0xB8, 0x96}, 3},    // Complete "世"
		{[]byte{}, 0},
	}

	for _, tc := range tests {
		result := lastRuneIndex(tc.input)
		if result != tc.expected {
			t.Errorf("lastRuneIndex(%q) = %d; want %d", tc.input, result, tc.expected)
		}
	}
}

func TestSegmentBoundaryIndex(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"Hello。World", 8},  // 5 (Hello) + 3 (UTF-8 len of 。) = 8
		{"Hello,World", 6},   // ASCII comma position + 1
		{"Hello World", 0},   // No boundary
		{"9.9 dollars", 0},   // Decimal point, not boundary
		{"10:30 time", 0},    // Time, not boundary
		{"First！Second", 8}, // 5 (First) + 3 (UTF-8 len of ！) = 8
	}

	for _, tc := range tests {
		result := segmentBoundaryIndex([]byte(tc.input))
		if result != tc.expected {
			t.Errorf("segmentBoundaryIndex(%q) = %d; want %d", tc.input, result, tc.expected)
		}
	}
}

func TestLastSegmentBoundaryIndex(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"First。Second。Third", 17}, // Last 。 position: 5 + 3 + 6 + 3 = 17
		{"Hello,World,End", 12},      // Last comma position
		{"No boundary here", 0},
	}

	for _, tc := range tests {
		result := lastSegmentBoundaryIndex([]byte(tc.input))
		if result != tc.expected {
			t.Errorf("lastSegmentBoundaryIndex(%q) = %d; want %d", tc.input, result, tc.expected)
		}
	}
}
