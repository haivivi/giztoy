package voiceprint

// Detector uses a sliding window of voice hashes to determine
// speaker status. It tracks the most recent hashes and classifies
// the current state as Single, Overlap, or Unknown based on hash
// distribution within the window.
//
// # Algorithm
//
// The detector maintains a circular buffer of the last N hashes.
// On each Feed() call it counts how many distinct hashes appear:
//
//   - 1 dominant hash with high ratio → StatusSingle
//   - 2 dominant hashes → StatusOverlap
//   - 3+ hashes or insufficient data → StatusUnknown
//
// The confidence value reflects how stable the window is:
// for Single, it's the ratio of the dominant hash count to window size;
// for Overlap, it's the ratio of the top-2 hashes to window size.
type Detector struct {
	window []string // circular buffer of recent hashes
	pos    int      // next write position
	filled int      // number of slots filled (up to len(window))

	// minRatio is the minimum fraction of the window that the dominant
	// hash must occupy to be considered "stable". Default: 0.6.
	minRatio float32
}

// DetectorOption configures a Detector.
type DetectorOption func(*Detector)

// WithWindowSize sets the sliding window size (default 5).
func WithWindowSize(n int) DetectorOption {
	return func(d *Detector) {
		if n > 0 {
			d.window = make([]string, n)
		}
	}
}

// WithMinRatio sets the minimum dominance ratio for Single detection
// (default 0.6). Must be in (0, 1].
func WithMinRatio(r float32) DetectorOption {
	return func(d *Detector) {
		if r > 0 && r <= 1 {
			d.minRatio = r
		}
	}
}

// NewDetector creates a Detector with the given options.
func NewDetector(opts ...DetectorOption) *Detector {
	d := &Detector{
		window:   make([]string, 5),
		minRatio: 0.6,
	}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

// Feed adds a new hash to the window and returns the current speaker state.
// Returns nil if the window has fewer than 2 entries (insufficient data).
func (d *Detector) Feed(hash string) *SpeakerChunk {
	// Write to circular buffer.
	d.window[d.pos] = hash
	d.pos = (d.pos + 1) % len(d.window)
	if d.filled < len(d.window) {
		d.filled++
	}

	// Need at least 2 samples to make a decision.
	if d.filled < 2 {
		return nil
	}

	// Count hash frequencies in the window.
	counts := make(map[string]int, 4)
	for i := range d.filled {
		idx := (d.pos - d.filled + i + len(d.window)) % len(d.window)
		counts[d.window[idx]]++
	}

	// Find top-1 and top-2 hashes.
	var top1Hash, top2Hash string
	var top1Count, top2Count int
	for h, c := range counts {
		if c > top1Count {
			top2Hash = top1Hash
			top2Count = top1Count
			top1Hash = h
			top1Count = c
		} else if c > top2Count {
			top2Hash = h
			top2Count = c
		}
	}

	total := float32(d.filled)

	// Single speaker: dominant hash exceeds minRatio.
	if float32(top1Count)/total >= d.minRatio {
		return &SpeakerChunk{
			Status:     StatusSingle,
			Speaker:    VoiceLabel(top1Hash),
			Candidates: []string{VoiceLabel(top1Hash)},
			Confidence: float32(top1Count) / total,
		}
	}

	// Overlap: two hashes together cover most of the window.
	if top2Count > 0 {
		combinedRatio := float32(top1Count+top2Count) / total
		if combinedRatio >= d.minRatio {
			return &SpeakerChunk{
				Status:     StatusOverlap,
				Speaker:    VoiceLabel(top1Hash),
				Candidates: []string{VoiceLabel(top1Hash), VoiceLabel(top2Hash)},
				Confidence: combinedRatio,
			}
		}
	}

	// Unknown: too many distinct hashes, unstable.
	return &SpeakerChunk{
		Status:     StatusUnknown,
		Confidence: float32(top1Count) / total,
	}
}

// Reset clears the window state.
func (d *Detector) Reset() {
	d.pos = 0
	d.filled = 0
	for i := range d.window {
		d.window[i] = ""
	}
}
