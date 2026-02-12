package memory

import (
	"context"
	"errors"
	"time"

	"github.com/haivivi/giztoy/go/pkg/kv"
)

// LongTerm provides multi-granularity time compression summaries for a
// persona's memory. Summaries are stored at increasing time scales:
//
//	Hour → Day → Week → Month → Year → Life
//
// Each summary is keyed by grain + time bucket (e.g., "hour" + "2026021315"
// for the 3 PM hour on Feb 13, 2026). The life summary is a single entry
// capturing the persona's entire existence.
//
// LongTerm does not generate summaries itself — the upper-layer [Compressor]
// produces them. LongTerm only stores and retrieves.
type LongTerm struct {
	store kv.Store
	mid   string // memory ID for key scoping
}

func newLongTerm(store kv.Store, mid string) *LongTerm {
	return &LongTerm{store: store, mid: mid}
}

// GetSummary retrieves the summary for a specific grain and time.
// Returns ("", nil) if no summary exists for the given bucket.
func (l *LongTerm) GetSummary(ctx context.Context, grain Grain, t time.Time) (string, error) {
	if grain == GrainLife {
		return l.LifeSummary(ctx)
	}
	tk, err := grainTimeKey(grain, t.UnixNano())
	if err != nil {
		return "", err
	}
	data, err := l.store.Get(ctx, longTermKey(l.mid, grain, tk))
	if err != nil {
		if errors.Is(err, kv.ErrNotFound) {
			return "", nil
		}
		return "", err
	}
	return string(data), nil
}

// SetSummary stores a summary for a specific grain and time.
func (l *LongTerm) SetSummary(ctx context.Context, grain Grain, t time.Time, summary string) error {
	if grain == GrainLife {
		return l.SetLifeSummary(ctx, summary)
	}
	tk, err := grainTimeKey(grain, t.UnixNano())
	if err != nil {
		return err
	}
	return l.store.Set(ctx, longTermKey(l.mid, grain, tk), []byte(summary))
}

// LifeSummary retrieves the life-level summary. Returns ("", nil) if none exists.
func (l *LongTerm) LifeSummary(ctx context.Context) (string, error) {
	data, err := l.store.Get(ctx, longTermLifeKey(l.mid))
	if err != nil {
		if errors.Is(err, kv.ErrNotFound) {
			return "", nil
		}
		return "", err
	}
	return string(data), nil
}

// SetLifeSummary stores the life-level summary.
func (l *LongTerm) SetLifeSummary(ctx context.Context, summary string) error {
	return l.store.Set(ctx, longTermLifeKey(l.mid), []byte(summary))
}

// Summaries lists all summaries at a given grain within a time range [from, to).
// Results are sorted chronologically (ascending by time key).
func (l *LongTerm) Summaries(ctx context.Context, grain Grain, from, to time.Time) ([]TimedSummary, error) {
	if grain == GrainLife {
		s, err := l.LifeSummary(ctx)
		if err != nil || s == "" {
			return nil, err
		}
		return []TimedSummary{{
			Grain:   GrainLife,
			Summary: s,
		}}, nil
	}

	fromKey, err := grainTimeKey(grain, from.UnixNano())
	if err != nil {
		return nil, err
	}
	toKey, err := grainTimeKey(grain, to.UnixNano())
	if err != nil {
		return nil, err
	}

	prefix := longTermPrefix(l.mid, grain)
	var results []TimedSummary

	for entry, err := range l.store.List(ctx, prefix) {
		if err != nil {
			return nil, err
		}
		// The time key is the last segment of the KV key.
		tk := entry.Key[len(entry.Key)-1]

		// Filter by range: fromKey <= tk < toKey (lexicographic).
		if tk < fromKey || tk >= toKey {
			continue
		}

		results = append(results, TimedSummary{
			Grain:   grain,
			Summary: string(entry.Value),
		})
	}

	return results, nil
}
