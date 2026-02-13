package memory

import (
	"context"
	"fmt"
	"strings"

	"github.com/haivivi/giztoy/go/pkg/genx/profilers"
	"github.com/haivivi/giztoy/go/pkg/genx/segmentors"
)

// LLMCompressor implements [Compressor] by delegating to the segmentors and
// profilers packages for LLM-based conversation compression.
//
// It calls [segmentors.Process] to extract entities, relations, and a
// compressed segment summary. Optionally, it calls [profilers.Process] to
// evolve entity schemas and update profiles.
//
// LLMCompressor is stateless and safe for concurrent use.
type LLMCompressor struct {
	cfg LLMCompressorConfig

	segMux  *segmentors.Mux
	profMux *profilers.Mux
}

// LLMCompressorConfig configures an [LLMCompressor].
type LLMCompressorConfig struct {
	// Segmentor is the pattern of the registered segmentor to use
	// (e.g., "seg/qwen-flash"). Required. Must be registered in
	// segmentors.DefaultMux (or the provided SegmentorMux).
	Segmentor string

	// Profiler is the pattern of the registered profiler to use
	// (e.g., "prof/qwen-flash"). Optional. If empty, profiling is
	// skipped and only segmentor output is used.
	Profiler string

	// Schema provides entity type hints to guide extraction.
	// Optional. If nil, the LLM discovers entity types freely.
	Schema *segmentors.Schema

	// Profiles holds current entity profiles for the profiler to
	// reference when proposing updates. Optional. Keyed by entity
	// label (e.g., "person:小明") → attribute map.
	Profiles map[string]map[string]any

	// SegmentorMux overrides segmentors.DefaultMux. Optional.
	SegmentorMux *segmentors.Mux

	// ProfilerMux overrides profilers.DefaultMux. Optional.
	ProfilerMux *profilers.Mux
}

// NewLLMCompressor creates a new LLM-based compressor.
func NewLLMCompressor(cfg LLMCompressorConfig) (*LLMCompressor, error) {
	if cfg.Segmentor == "" {
		return nil, fmt.Errorf("memory: LLMCompressorConfig.Segmentor is required")
	}
	return &LLMCompressor{
		cfg:     cfg,
		segMux:  cfg.SegmentorMux,
		profMux: cfg.ProfilerMux,
	}, nil
}

// CompressMessages compresses conversation messages into memory segments
// and an updated summary by calling the segmentor LLM.
func (c *LLMCompressor) CompressMessages(ctx context.Context, messages []Message) (*CompressResult, error) {
	result, err := c.runSegmentor(ctx, messages)
	if err != nil {
		return nil, err
	}

	// Convert segmentor output to CompressResult.
	seg := SegmentInput{
		Summary:  result.Segment.Summary,
		Keywords: result.Segment.Keywords,
		Labels:   result.Segment.Labels,
	}

	return &CompressResult{
		Segments: []SegmentInput{seg},
		Summary:  result.Segment.Summary,
	}, nil
}

// ExtractEntities extracts entity and relation updates from messages
// by calling the segmentor (and optionally the profiler) LLM.
func (c *LLMCompressor) ExtractEntities(ctx context.Context, messages []Message) (*EntityUpdate, error) {
	result, err := c.runSegmentor(ctx, messages)
	if err != nil {
		return nil, err
	}

	update := convertSegmentorResult(result)

	// Run profiler if configured.
	if c.cfg.Profiler != "" {
		profResult, err := c.runProfiler(ctx, messages, result)
		if err != nil {
			// Profiler failure is non-fatal — we still have the segmentor result.
			// Log and continue (the caller may decide to log this).
			_ = err
		} else {
			mergeProfilerResult(update, profResult)
		}
	}

	return update, nil
}

// runSegmentor calls segmentors.Process with the converted messages.
func (c *LLMCompressor) runSegmentor(ctx context.Context, messages []Message) (*segmentors.Result, error) {
	input := segmentors.Input{
		Messages: messagesToStrings(messages),
		Schema:   c.cfg.Schema,
	}

	if c.segMux != nil {
		return c.segMux.Process(ctx, c.cfg.Segmentor, input)
	}
	return segmentors.Process(ctx, c.cfg.Segmentor, input)
}

// runProfiler calls profilers.Process with the segmentor result and context.
func (c *LLMCompressor) runProfiler(ctx context.Context, messages []Message, segResult *segmentors.Result) (*profilers.Result, error) {
	input := profilers.Input{
		Messages:  messagesToStrings(messages),
		Extracted: segResult,
		Schema:    c.cfg.Schema,
		Profiles:  c.cfg.Profiles,
	}

	if c.profMux != nil {
		return c.profMux.Process(ctx, c.cfg.Profiler, input)
	}
	return profilers.Process(ctx, c.cfg.Profiler, input)
}

// messagesToStrings converts memory.Message slice to the plain string format
// expected by segmentors/profilers: "role: content".
func messagesToStrings(messages []Message) []string {
	out := make([]string, 0, len(messages))
	for _, m := range messages {
		if m.Content == "" {
			continue
		}
		var sb strings.Builder
		sb.WriteString(string(m.Role))
		if m.Name != "" {
			sb.WriteByte('(')
			sb.WriteString(m.Name)
			sb.WriteByte(')')
		}
		sb.WriteString(": ")
		sb.WriteString(m.Content)
		out = append(out, sb.String())
	}
	return out
}

// convertSegmentorResult converts segmentor entities and relations into
// a memory.EntityUpdate.
func convertSegmentorResult(result *segmentors.Result) *EntityUpdate {
	update := &EntityUpdate{}

	for _, e := range result.Entities {
		update.Entities = append(update.Entities, EntityInput{
			Label: e.Label,
			Attrs: e.Attrs,
		})
	}

	for _, r := range result.Relations {
		update.Relations = append(update.Relations, RelationInput{
			From:    r.From,
			To:      r.To,
			RelType: r.RelType,
		})
	}

	return update
}

// mergeProfilerResult merges profiler output into an existing EntityUpdate.
// Profile updates are merged as entity attrs; additional relations are appended.
func mergeProfilerResult(update *EntityUpdate, profResult *profilers.Result) {
	if profResult == nil {
		return
	}

	// Merge profile updates as entity attrs.
	for label, attrs := range profResult.ProfileUpdates {
		found := false
		for i, e := range update.Entities {
			if e.Label == label {
				// Merge attrs into existing entity.
				if update.Entities[i].Attrs == nil {
					update.Entities[i].Attrs = make(map[string]any)
				}
				for k, v := range attrs {
					update.Entities[i].Attrs[k] = v
				}
				found = true
				break
			}
		}
		if !found {
			update.Entities = append(update.Entities, EntityInput{
				Label: label,
				Attrs: attrs,
			})
		}
	}

	// Append additional relations from profiler.
	for _, r := range profResult.Relations {
		update.Relations = append(update.Relations, RelationInput{
			From:    r.From,
			To:      r.To,
			RelType: r.RelType,
		})
	}
}
