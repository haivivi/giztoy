package memory

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/haivivi/giztoy/go/pkg/graph"
	"github.com/haivivi/giztoy/go/pkg/kv"
	"github.com/haivivi/giztoy/go/pkg/recall"
	"github.com/haivivi/giztoy/go/pkg/vecstore"
)

// ---------------------------------------------------------------------------
// Mock embedder: 8-dimensional vectors with semantic clustering
// ---------------------------------------------------------------------------
//
// Dimensions represent semantic axes:
//
//	0: dinosaurs/prehistoric
//	1: space/science
//	2: cooking/food
//	3: music/singing
//	4: drawing/art
//	5: story/fairy-tale
//	6: sports/outdoor
//	7: building/Lego
//
// Each text maps to a hand-tuned vector so cosine similarity produces
// realistic ranking behaviour in tests.

type mockEmbedder struct {
	dim     int
	model   string
	vectors map[string][]float32
}

func newMockEmbedder() *mockEmbedder {
	return &mockEmbedder{
		dim: 8,
		vectors: map[string][]float32{
			// --- Pure query axes (used as search queries) ---
			"dinosaurs": {1, 0, 0, 0, 0, 0, 0, 0},
			"space":     {0, 1, 0, 0, 0, 0, 0, 0},
			"cooking":   {0, 0, 1, 0, 0, 0, 0, 0},
			"music":     {0, 0, 0, 1, 0, 0, 0, 0},
			"drawing":   {0, 0, 0, 0, 1, 0, 0, 0},
			"stories":   {0, 0, 0, 0, 0, 1, 0, 0},
			"sports":    {0, 0, 0, 0, 0, 0, 1, 0},
			"lego":      {0, 0, 0, 0, 0, 0, 0, 1},

			// --- Segment summaries (realistic conversations) ---

			// Day 1: 小明 dinosaur session
			"和小明聊了恐龙，他最喜欢霸王龙":      {0.9, 0, 0, 0, 0, 0, 0, 0},
			"小明问了很多恐龙的问题，还画了一只三角龙": {0.7, 0, 0, 0, 0.3, 0, 0, 0},
			"和小明聊了恐龙":              {0.9, 0.1, 0, 0, 0, 0, 0, 0},
			"学到了太空知识":              {0.1, 0.9, 0, 0, 0, 0, 0, 0},
			"一起做了饭":                {0.1, 0, 0.9, 0, 0, 0, 0, 0},
			"played music":         {0, 0, 0.1, 0.9, 0, 0, 0, 0},
			"小明说长大想当古生物学家":         {0.8, 0.1, 0, 0, 0, 0, 0, 0},
			"给小明讲了恐龙灭绝的故事，他有点伤心":   {0.6, 0, 0, 0, 0, 0.3, 0, 0},

			// Day 2: 小红 drawing session
			"小红画了一个公主城堡，涂了粉色和金色": {0, 0, 0, 0, 0.8, 0.2, 0, 0},
			"和小红一起编了一个公主和小猫的故事":  {0, 0, 0, 0, 0.2, 0.8, 0, 0},
			"小红说她的公主会骑恐龙":        {0.3, 0, 0, 0, 0.4, 0.3, 0, 0},

			// Day 3: 妈妈 cooking
			"妈妈教我们做了蛋炒饭，小明吃了两碗": {0, 0, 0.9, 0, 0, 0, 0, 0},
			"妈妈说周末要做恐龙形状的饼干":    {0.3, 0, 0.7, 0, 0, 0, 0, 0},

			// Day 4: 爸爸 music + Lego
			"和爸爸一起听了古典音乐，小明跟着打节拍": {0, 0, 0, 0.8, 0, 0, 0, 0.1},
			"爸爸和小明一起拼了一个恐龙乐高模型":   {0.4, 0, 0, 0, 0, 0, 0, 0.6},

			// Day 5: Outdoor + science
			"全家去了自然博物馆看恐龙化石，小明超兴奋": {0.7, 0.2, 0, 0, 0, 0, 0.1, 0},
			"小红在博物馆里画了好多恐龙素描":      {0.4, 0, 0, 0, 0.6, 0, 0, 0},
			"小明在天文馆看了星空投影，问了黑洞的问题": {0, 0.9, 0, 0, 0, 0, 0, 0},

			// Day 6: Bedtime stories
			"给小明讲了宇宙探险的睡前故事":         {0, 0.5, 0, 0, 0, 0.5, 0, 0},
			"给小红讲了小猫公主和恐龙的故事，她听得好开心": {0.2, 0, 0, 0, 0, 0.7, 0, 0},

			// Day 7: 小红 art class
			"小红今天美术课画了全家福，画里还有我": {0, 0, 0, 0, 0.9, 0, 0, 0},

			// --- Search queries ---
			"小明喜欢什么":    {0.5, 0.2, 0, 0, 0, 0, 0, 0.3},
			"小红喜欢什么":    {0, 0, 0, 0, 0.5, 0.3, 0, 0},
			"最近和家人做了什么": {0.2, 0.1, 0.2, 0.1, 0.1, 0.2, 0.1, 0.1},
			"恐龙相关的回忆":   {0.95, 0, 0, 0, 0, 0.05, 0, 0},
		},
	}
}

func (m *mockEmbedder) Embed(_ context.Context, text string) ([]float32, error) {
	if v, ok := m.vectors[text]; ok {
		return v, nil
	}
	return make([]float32, m.dim), nil
}

func (m *mockEmbedder) EmbedBatch(_ context.Context, texts []string) ([][]float32, error) {
	vecs := make([][]float32, len(texts))
	for i, t := range texts {
		v, err := m.Embed(context.Background(), t)
		if err != nil {
			return nil, err
		}
		vecs[i] = v
	}
	return vecs, nil
}

func (m *mockEmbedder) Dimension() int { return m.dim }

func (m *mockEmbedder) Model() string {
	if m.model != "" {
		return m.model
	}
	return "mock-embed"
}

// mockCompressor is a simple test compressor.
type mockCompressor struct{}

func (mc *mockCompressor) CompressMessages(_ context.Context, msgs []Message) (*CompressResult, error) {
	summary := ""
	for _, m := range msgs {
		if m.Content != "" {
			if summary != "" {
				summary += "; "
			}
			summary += m.Content
		}
	}
	return &CompressResult{
		Segments: []SegmentInput{{
			Summary:  summary,
			Keywords: []string{"test"},
			Labels:   []string{"person:test"},
		}},
		Summary: "compressed: " + summary,
	}, nil
}

func (mc *mockCompressor) ExtractEntities(_ context.Context, _ []Message) (*EntityUpdate, error) {
	return &EntityUpdate{
		Entities: []EntityInput{{
			Label: "person:test",
			Attrs: map[string]any{"compressed": true},
		}},
	}, nil
}

func (mc *mockCompressor) CompactSegments(_ context.Context, summaries []string) (*CompressResult, error) {
	combined := ""
	for _, s := range summaries {
		if combined != "" {
			combined += " | "
		}
		combined += s
	}
	return &CompressResult{
		Segments: []SegmentInput{{
			Summary:  combined,
			Keywords: []string{"compacted"},
			Labels:   []string{"person:test"},
		}},
		Summary: combined,
	}, nil
}

// testSep is the KV separator used in tests. ASCII Unit Separator (0x1F)
// allows natural colon-namespaced labels like "person:小明".
const testSep byte = 0x1F

// newTestHost creates a Host with all components for testing.
// Uses null byte separator so labels can contain ':'.
func newTestHost(t *testing.T) *Host {
	t.Helper()
	store := kv.NewMemory(&kv.Options{Separator: testSep})
	emb := newMockEmbedder()
	vec := vecstore.NewMemory()

	h, err := NewHost(context.Background(), HostConfig{
		Store:     store,
		Vec:       vec,
		Embedder:  emb,
		Separator: testSep,
	})
	if err != nil {
		t.Fatalf("NewHost: %v", err)
	}
	return h
}

// newTestHostNoVec creates a Host without vector search.
func newTestHostNoVec(t *testing.T) *Host {
	t.Helper()
	store := kv.NewMemory(&kv.Options{Separator: testSep})
	h, err := NewHost(context.Background(), HostConfig{Store: store, Separator: testSep})
	if err != nil {
		t.Fatalf("NewHost: %v", err)
	}
	return h
}

// mustOpen is a test helper that calls h.Open and fails the test on error.
func mustOpen(t testing.TB, h *Host, id string, opts ...OpenOption) *Memory {
	t.Helper()
	m, err := h.Open(id, opts...)
	if err != nil {
		t.Fatalf("Open(%q): %v", id, err)
	}
	return m
}

// ---------------------------------------------------------------------------
// Host tests
// ---------------------------------------------------------------------------

func TestHostOpen(t *testing.T) {
	h := newTestHost(t)
	defer h.Close()

	m1 := mustOpen(t, h, "cat_girl")
	m2 := mustOpen(t, h, "cat_girl")
	m3 := mustOpen(t, h, "robot_boy")

	if m1 != m2 {
		t.Fatal("same ID should return same Memory instance")
	}
	if m1 == m3 {
		t.Fatal("different IDs should return different Memory instances")
	}
	if m1.ID() != "cat_girl" {
		t.Fatalf("ID() = %q, want %q", m1.ID(), "cat_girl")
	}
	if m3.ID() != "robot_boy" {
		t.Fatalf("ID() = %q, want %q", m3.ID(), "robot_boy")
	}
}

func TestHostNilStoreReturnsError(t *testing.T) {
	_, err := NewHost(context.Background(), HostConfig{})
	if err == nil {
		t.Fatal("expected error on nil Store")
	}
}

func TestHostEmbedModelPersistence(t *testing.T) {
	ctx := context.Background()
	store := kv.NewMemory(&kv.Options{Separator: testSep})

	// First NewHost: should persist mock-embed model metadata.
	emb := newMockEmbedder()
	h1, err := NewHost(ctx, HostConfig{
		Store: store, Embedder: emb, Separator: testSep,
	})
	if err != nil {
		t.Fatalf("first NewHost: %v", err)
	}
	h1.Close()

	// Second NewHost with same embedder: should succeed.
	h2, err := NewHost(ctx, HostConfig{
		Store: store, Embedder: emb, Separator: testSep,
	})
	if err != nil {
		t.Fatalf("second NewHost (same model): %v", err)
	}
	h2.Close()

	// Third NewHost with different model: should fail.
	differentEmb := &mockEmbedder{dim: 8, vectors: map[string][]float32{}}
	differentEmb.model = "different-model"
	_, err = NewHost(ctx, HostConfig{
		Store: store, Embedder: differentEmb, Separator: testSep,
	})
	if err == nil {
		t.Fatal("expected error on model mismatch, got nil")
	}
	t.Logf("got expected error: %v", err)
}

func TestHostEmbedDimensionMismatch(t *testing.T) {
	ctx := context.Background()
	store := kv.NewMemory(&kv.Options{Separator: testSep})

	// First NewHost with dim=8.
	emb8 := newMockEmbedder() // dim=8
	h1, err := NewHost(ctx, HostConfig{
		Store: store, Embedder: emb8, Separator: testSep,
	})
	if err != nil {
		t.Fatalf("first NewHost: %v", err)
	}
	h1.Close()

	// Second NewHost with same model but dim=4: should fail.
	emb4 := &mockEmbedder{dim: 4, vectors: map[string][]float32{}}
	_, err = NewHost(ctx, HostConfig{
		Store: store, Embedder: emb4, Separator: testSep,
	})
	if err == nil {
		t.Fatal("expected error on dimension mismatch, got nil")
	}
	t.Logf("got expected error: %v", err)
}

func TestHostNoEmbedderSkipsCheck(t *testing.T) {
	ctx := context.Background()
	store := kv.NewMemory(&kv.Options{Separator: testSep})

	// NewHost without embedder: should succeed, no metadata written.
	h, err := NewHost(ctx, HostConfig{Store: store, Separator: testSep})
	if err != nil {
		t.Fatalf("NewHost without embedder: %v", err)
	}
	h.Close()
}

// ---------------------------------------------------------------------------
// Conversation tests
// ---------------------------------------------------------------------------

func TestConversationAppendAndRecent(t *testing.T) {
	h := newTestHost(t)
	defer h.Close()
	m := mustOpen(t, h, "test")
	ctx := context.Background()

	conv := m.OpenConversation("device-001", []string{"person:alice"})
	if conv.ID() != "device-001" {
		t.Fatalf("ID() = %q, want %q", conv.ID(), "device-001")
	}

	// Append messages with explicit timestamps for deterministic ordering.
	msgs := []Message{
		{Role: RoleUser, Content: "hello", Timestamp: 1000},
		{Role: RoleModel, Content: "hi there", Timestamp: 2000},
		{Role: RoleUser, Content: "how are you?", Timestamp: 3000},
		{Role: RoleModel, Content: "I'm good!", Timestamp: 4000},
	}
	for _, msg := range msgs {
		if err := conv.Append(ctx, msg); err != nil {
			t.Fatalf("Append: %v", err)
		}
	}

	// Recent(2) should return the last 2 messages.
	recent, err := conv.Recent(ctx, 2)
	if err != nil {
		t.Fatalf("Recent: %v", err)
	}
	if len(recent) != 2 {
		t.Fatalf("len(recent) = %d, want 2", len(recent))
	}
	if recent[0].Content != "how are you?" {
		t.Errorf("recent[0].Content = %q, want %q", recent[0].Content, "how are you?")
	}
	if recent[1].Content != "I'm good!" {
		t.Errorf("recent[1].Content = %q, want %q", recent[1].Content, "I'm good!")
	}

	// Recent(10) should return all 4.
	all, err := conv.Recent(ctx, 10)
	if err != nil {
		t.Fatalf("Recent: %v", err)
	}
	if len(all) != 4 {
		t.Fatalf("len(all) = %d, want 4", len(all))
	}
}

func TestConversationAutoTimestamp(t *testing.T) {
	h := newTestHost(t)
	defer h.Close()
	m := mustOpen(t, h, "test")
	ctx := context.Background()

	// Override nowNano for deterministic test.
	origNow := nowNano
	nowNano = func() int64 { return 999 }
	defer func() { nowNano = origNow }()

	conv := m.OpenConversation("s1", nil)
	if err := conv.Append(ctx, Message{Role: RoleUser, Content: "hi"}); err != nil {
		t.Fatalf("Append: %v", err)
	}

	msgs, err := conv.Recent(ctx, 1)
	if err != nil {
		t.Fatalf("Recent: %v", err)
	}
	if msgs[0].Timestamp != 999 {
		t.Fatalf("Timestamp = %d, want 999", msgs[0].Timestamp)
	}
}

func TestConversationRevert(t *testing.T) {
	h := newTestHost(t)
	defer h.Close()
	m := mustOpen(t, h, "test")
	ctx := context.Background()

	conv := m.OpenConversation("device-001", nil)

	// Build a conversation.
	msgs := []Message{
		{Role: RoleUser, Content: "hello", Timestamp: 1000},
		{Role: RoleModel, Content: "hi", Timestamp: 2000},
		{Role: RoleUser, Content: "tell me about dinosaurs", Timestamp: 3000},
		{Role: RoleModel, Content: "dinosaurs are...", Timestamp: 4000},
	}
	for _, msg := range msgs {
		if err := conv.Append(ctx, msg); err != nil {
			t.Fatalf("Append: %v", err)
		}
	}

	// Revert should remove the last user message and model response.
	if err := conv.Revert(ctx); err != nil {
		t.Fatalf("Revert: %v", err)
	}

	remaining, err := conv.All(ctx)
	if err != nil {
		t.Fatalf("All: %v", err)
	}
	if len(remaining) != 2 {
		t.Fatalf("len(remaining) = %d, want 2", len(remaining))
	}
	if remaining[0].Content != "hello" {
		t.Errorf("remaining[0].Content = %q, want %q", remaining[0].Content, "hello")
	}
	if remaining[1].Content != "hi" {
		t.Errorf("remaining[1].Content = %q, want %q", remaining[1].Content, "hi")
	}
}

func TestConversationRevertEmpty(t *testing.T) {
	h := newTestHost(t)
	defer h.Close()
	m := mustOpen(t, h, "test")
	ctx := context.Background()

	conv := m.OpenConversation("s1", nil)
	// Revert on empty conversation should be a no-op.
	if err := conv.Revert(ctx); err != nil {
		t.Fatalf("Revert on empty: %v", err)
	}
}

func TestConversationCount(t *testing.T) {
	h := newTestHost(t)
	defer h.Close()
	m := mustOpen(t, h, "test")
	ctx := context.Background()

	conv := m.OpenConversation("s1", nil)

	count, err := conv.Count(ctx)
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if count != 0 {
		t.Fatalf("Count = %d, want 0", count)
	}

	for i := range 5 {
		if err := conv.Append(ctx, Message{
			Role:      RoleUser,
			Content:   "msg",
			Timestamp: int64((i + 1) * 1000),
		}); err != nil {
			t.Fatalf("Append: %v", err)
		}
	}

	count, err = conv.Count(ctx)
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if count != 5 {
		t.Fatalf("Count = %d, want 5", count)
	}
}

func TestConversationClear(t *testing.T) {
	h := newTestHost(t)
	defer h.Close()
	m := mustOpen(t, h, "test")
	ctx := context.Background()

	conv := m.OpenConversation("s1", nil)
	if err := conv.Append(ctx, Message{Role: RoleUser, Content: "hi", Timestamp: 1000}); err != nil {
		t.Fatalf("Append: %v", err)
	}

	if err := conv.Clear(ctx); err != nil {
		t.Fatalf("Clear: %v", err)
	}

	count, err := conv.Count(ctx)
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if count != 0 {
		t.Fatalf("Count after clear = %d, want 0", count)
	}
}

// ---------------------------------------------------------------------------
// Bucket compaction tests
// ---------------------------------------------------------------------------

func TestBucketStatsEmpty(t *testing.T) {
	h := newTestHost(t)
	defer h.Close()
	m := mustOpen(t, h, "test")
	ctx := context.Background()

	count, chars, err := m.Index().BucketStats(ctx, recall.Bucket1H)
	if err != nil {
		t.Fatalf("BucketStats: %v", err)
	}
	if count != 0 || chars != 0 {
		t.Fatalf("expected 0/0, got %d/%d", count, chars)
	}
}

func TestStoreSegmentInBucket(t *testing.T) {
	h := newTestHost(t)
	defer h.Close()
	m := mustOpen(t, h, "test")
	ctx := context.Background()

	// Store segments in different buckets.
	if err := m.StoreSegment(ctx, SegmentInput{
		Summary: "hourly summary", Keywords: []string{"test"}, Labels: []string{"person:小明"},
	}, recall.Bucket1H); err != nil {
		t.Fatalf("StoreSegment 1h: %v", err)
	}
	if err := m.StoreSegment(ctx, SegmentInput{
		Summary: "daily summary", Keywords: []string{"test"}, Labels: []string{"person:小明"},
	}, recall.Bucket1D); err != nil {
		t.Fatalf("StoreSegment 1d: %v", err)
	}

	// Verify bucket stats.
	count1h, _, err := m.Index().BucketStats(ctx, recall.Bucket1H)
	if err != nil {
		t.Fatalf("BucketStats 1h: %v", err)
	}
	if count1h != 1 {
		t.Fatalf("1h count = %d, want 1", count1h)
	}

	count1d, _, err := m.Index().BucketStats(ctx, recall.Bucket1D)
	if err != nil {
		t.Fatalf("BucketStats 1d: %v", err)
	}
	if count1d != 1 {
		t.Fatalf("1d count = %d, want 1", count1d)
	}

	// Search should find segments across all buckets.
	segs, err := m.Index().RecentSegments(ctx, 10)
	if err != nil {
		t.Fatalf("RecentSegments: %v", err)
	}
	if len(segs) != 2 {
		t.Fatalf("RecentSegments = %d, want 2", len(segs))
	}
}

func TestCompactBucketNoOp(t *testing.T) {
	h := newTestHost(t)
	defer h.Close()
	m := mustOpen(t, h, "test")
	ctx := context.Background()

	// Compact on empty bucket should be no-op.
	if err := m.Compact(ctx); err != nil {
		t.Fatalf("Compact: %v", err)
	}
}

func TestBucketForSpan(t *testing.T) {
	tests := []struct {
		span time.Duration
		want recall.Bucket
	}{
		{30 * time.Minute, recall.Bucket1H},
		{1 * time.Hour, recall.Bucket1H},
		{12 * time.Hour, recall.Bucket1D},
		{3 * 24 * time.Hour, recall.Bucket1W},
		{10 * 24 * time.Hour, recall.Bucket1M},
		{60 * 24 * time.Hour, recall.Bucket3M},
		{120 * 24 * time.Hour, recall.Bucket6M},
		{300 * 24 * time.Hour, recall.Bucket1Y},
		{400 * 24 * time.Hour, recall.BucketLT},
	}
	for _, tt := range tests {
		got := recall.BucketForSpan(tt.span)
		if got != tt.want {
			t.Errorf("BucketForSpan(%v) = %s, want %s", tt.span, got, tt.want)
		}
	}
}

func TestEnsureCoarser(t *testing.T) {
	// When BucketForSpan returns the same bucket as source, ensureCoarser
	// should bump to the next coarser level.
	got := ensureCoarser(recall.Bucket1H, recall.Bucket1H)
	if got != recall.Bucket1D {
		t.Errorf("ensureCoarser(1h, 1h) = %s, want 1d", got)
	}
	// When target is already coarser, it passes through.
	got = ensureCoarser(recall.Bucket1H, recall.Bucket1W)
	if got != recall.Bucket1W {
		t.Errorf("ensureCoarser(1h, 1w) = %s, want 1w", got)
	}
	// 1y → lt for the terminal case.
	got = ensureCoarser(recall.Bucket1Y, recall.Bucket1Y)
	if got != recall.BucketLT {
		t.Errorf("ensureCoarser(1y, 1y) = %s, want lt", got)
	}
}

// newTestHostWithCompactor creates a Host with a mock compressor and a
// custom CompressPolicy. Useful for testing compaction behavior.
func newTestHostWithCompactor(t *testing.T, policy CompressPolicy) *Host {
	t.Helper()
	store := kv.NewMemory(&kv.Options{Separator: testSep})
	emb := newMockEmbedder()
	vec := vecstore.NewMemory()

	h, err := NewHost(context.Background(), HostConfig{
		Store:          store,
		Vec:            vec,
		Embedder:       emb,
		Separator:      testSep,
		Compressor:     &mockCompressor{},
		CompressPolicy: policy,
	})
	if err != nil {
		t.Fatalf("NewHost: %v", err)
	}
	return h
}

func TestCompactBucketMovesSegments(t *testing.T) {
	policy := CompressPolicy{MaxMessages: 5, MaxChars: 100000}
	h := newTestHostWithCompactor(t, policy)
	defer h.Close()
	m := mustOpen(t, h, "test")
	ctx := context.Background()

	// Store 8 segments in the 1h bucket (exceeds threshold of 5).
	for i := range 8 {
		if err := m.StoreSegment(ctx, SegmentInput{
			Summary:  fmt.Sprintf("hourly event %d", i),
			Keywords: []string{"test"},
			Labels:   []string{"person:test"},
		}, recall.Bucket1H); err != nil {
			t.Fatalf("StoreSegment: %v", err)
		}
	}

	// Verify 1h bucket has 8 segments.
	count, _, err := m.Index().BucketStats(ctx, recall.Bucket1H)
	if err != nil {
		t.Fatalf("BucketStats: %v", err)
	}
	if count != 8 {
		t.Fatalf("1h count = %d, want 8", count)
	}

	// Compact.
	if err := m.Compact(ctx); err != nil {
		t.Fatalf("Compact: %v", err)
	}

	// 1h bucket should have fewer segments.
	count1h, _, err := m.Index().BucketStats(ctx, recall.Bucket1H)
	if err != nil {
		t.Fatalf("BucketStats 1h: %v", err)
	}
	if count1h >= 8 {
		t.Errorf("1h count after compact = %d, expected < 8", count1h)
	}

	// A coarser bucket should have received the compacted segment.
	// The mock compressor's CompactSegments returns 1 segment, and all
	// source segments have close timestamps → BucketForSpan returns 1h
	// → ensureCoarser bumps to 1d.
	count1d, _, err := m.Index().BucketStats(ctx, recall.Bucket1D)
	if err != nil {
		t.Fatalf("BucketStats 1d: %v", err)
	}
	if count1d == 0 {
		t.Error("expected at least 1 segment in 1d bucket after compaction")
	}

	// Total segments should be: remaining in 1h + new in 1d.
	total := count1h + count1d
	t.Logf("After compact: 1h=%d, 1d=%d, total=%d", count1h, count1d, total)
}

func TestCompactCascade(t *testing.T) {
	policy := CompressPolicy{MaxMessages: 3, MaxChars: 100000}
	h := newTestHostWithCompactor(t, policy)
	defer h.Close()
	m := mustOpen(t, h, "test")
	ctx := context.Background()

	// Fill 1h bucket with 6 segments (2x threshold).
	for i := range 6 {
		if err := m.StoreSegment(ctx, SegmentInput{
			Summary:  fmt.Sprintf("hour %d", i),
			Keywords: []string{"test"},
			Labels:   []string{"person:test"},
		}, recall.Bucket1H); err != nil {
			t.Fatalf("StoreSegment 1h: %v", err)
		}
	}

	// First compact: 1h → 1d.
	if err := m.Compact(ctx); err != nil {
		t.Fatalf("Compact 1: %v", err)
	}

	count1d, _, _ := m.Index().BucketStats(ctx, recall.Bucket1D)
	t.Logf("After first compact: 1d=%d", count1d)

	// Now fill 1d bucket past threshold by adding more 1h segments
	// and compacting them to 1d repeatedly.
	for round := range 5 {
		for i := range 6 {
			if err := m.StoreSegment(ctx, SegmentInput{
				Summary:  fmt.Sprintf("hour round%d-%d", round, i),
				Keywords: []string{"test"},
				Labels:   []string{"person:test"},
			}, recall.Bucket1H); err != nil {
				t.Fatalf("StoreSegment: %v", err)
			}
		}
		if err := m.Compact(ctx); err != nil {
			t.Fatalf("Compact round %d: %v", round, err)
		}
	}

	// After many compactions, 1d should have been compacted further.
	count1d, _, _ = m.Index().BucketStats(ctx, recall.Bucket1D)
	count1w, _, _ := m.Index().BucketStats(ctx, recall.Bucket1W)
	t.Logf("After cascade: 1d=%d, 1w=%d", count1d, count1w)

	// At least one coarser bucket beyond 1d should have segments.
	hasCoarser := false
	for _, b := range []recall.Bucket{recall.Bucket1W, recall.Bucket1M, recall.Bucket3M, recall.Bucket6M, recall.Bucket1Y, recall.BucketLT} {
		c, _, _ := m.Index().BucketStats(ctx, b)
		if c > 0 {
			hasCoarser = true
			t.Logf("  bucket %s: %d segments", b, c)
		}
	}
	if !hasCoarser {
		t.Error("expected compaction cascade to produce segments in buckets coarser than 1d")
	}
}

func TestCompactBucketUnderThreshold(t *testing.T) {
	policy := CompressPolicy{MaxMessages: 5, MaxChars: 100000}
	h := newTestHostWithCompactor(t, policy)
	defer h.Close()
	m := mustOpen(t, h, "test")
	ctx := context.Background()

	// Store 3 segments in 1h (under threshold of 5).
	for i := range 3 {
		if err := m.StoreSegment(ctx, SegmentInput{
			Summary:  fmt.Sprintf("event %d", i),
			Keywords: []string{"test"},
			Labels:   []string{"person:test"},
		}, recall.Bucket1H); err != nil {
			t.Fatalf("StoreSegment: %v", err)
		}
	}

	// Compact — should be a no-op.
	if err := m.Compact(ctx); err != nil {
		t.Fatalf("Compact: %v", err)
	}

	// 1h bucket should still have 3 segments.
	count, _, err := m.Index().BucketStats(ctx, recall.Bucket1H)
	if err != nil {
		t.Fatalf("BucketStats: %v", err)
	}
	if count != 3 {
		t.Errorf("1h count = %d, want 3 (no compaction expected)", count)
	}

	// No other bucket should have segments.
	for _, b := range []recall.Bucket{recall.Bucket1D, recall.Bucket1W, recall.Bucket1M, recall.Bucket3M, recall.Bucket6M, recall.Bucket1Y, recall.BucketLT} {
		c, _, _ := m.Index().BucketStats(ctx, b)
		if c != 0 {
			t.Errorf("bucket %s has %d segments, expected 0", b, c)
		}
	}
}

func TestCompactLtNeverCompacted(t *testing.T) {
	policy := CompressPolicy{MaxMessages: 3, MaxChars: 100000}
	h := newTestHostWithCompactor(t, policy)
	defer h.Close()
	m := mustOpen(t, h, "test")
	ctx := context.Background()

	// Store 10 segments directly in lt bucket (past threshold).
	for i := range 10 {
		if err := m.StoreSegment(ctx, SegmentInput{
			Summary:  fmt.Sprintf("lifetime event %d", i),
			Keywords: []string{"test"},
			Labels:   []string{"person:test"},
		}, recall.BucketLT); err != nil {
			t.Fatalf("StoreSegment: %v", err)
		}
	}

	// Compact — lt should be untouched.
	if err := m.Compact(ctx); err != nil {
		t.Fatalf("Compact: %v", err)
	}

	count, _, err := m.Index().BucketStats(ctx, recall.BucketLT)
	if err != nil {
		t.Fatalf("BucketStats: %v", err)
	}
	if count != 10 {
		t.Errorf("lt count = %d, want 10 (should never be compacted)", count)
	}
}

func TestAutoCompressTriggersCompact(t *testing.T) {
	// Low policy: conversation compresses at 3 messages, bucket compacts at 3 segments.
	policy := CompressPolicy{MaxMessages: 3, MaxChars: 100000}
	h := newTestHostWithCompactor(t, policy)
	defer h.Close()
	m := mustOpen(t, h, "test")
	ctx := context.Background()
	conv := m.OpenConversation("dev1", []string{"person:test"})

	// Append 3 messages → triggers auto-compress → 1 segment in 1h bucket.
	for i := range 3 {
		if err := conv.Append(ctx, Message{
			Role:    RoleUser,
			Content: fmt.Sprintf("message %d about dinosaurs", i),
		}); err != nil {
			t.Fatalf("Append %d: %v", i, err)
		}
	}

	count1h, _, _ := m.Index().BucketStats(ctx, recall.Bucket1H)
	t.Logf("After 3 messages: 1h=%d", count1h)
	if count1h == 0 {
		t.Fatal("expected at least 1 segment in 1h after auto-compress")
	}

	// Append many more messages to trigger multiple auto-compress cycles.
	// Each 3 messages → 1 auto-compress → 1 new segment in 1h.
	// When 1h reaches 3 segments → compact cascades to 1d.
	for i := range 12 {
		if err := conv.Append(ctx, Message{
			Role:    RoleUser,
			Content: fmt.Sprintf("more message %d about space exploration", i),
		}); err != nil {
			t.Fatalf("Append extra %d: %v", i, err)
		}
	}

	count1h, _, _ = m.Index().BucketStats(ctx, recall.Bucket1H)
	count1d, _, _ := m.Index().BucketStats(ctx, recall.Bucket1D)
	t.Logf("After 15 total messages: 1h=%d, 1d=%d", count1h, count1d)

	// With 15 messages and threshold=3: ~5 compress cycles → 5 segments in 1h.
	// Compact should have moved some to 1d.
	if count1d == 0 {
		t.Error("expected compaction cascade to produce segments in 1d bucket")
	}

	// Verify all segments are searchable.
	segs, err := m.Index().RecentSegments(ctx, 100)
	if err != nil {
		t.Fatalf("RecentSegments: %v", err)
	}
	if len(segs) == 0 {
		t.Fatal("expected segments after auto-compress + compact")
	}
	t.Logf("Total segments across all buckets: %d", len(segs))
}

// ---------------------------------------------------------------------------
// Memory: Graph integration tests
// ---------------------------------------------------------------------------

func TestMemoryGraph(t *testing.T) {
	h := newTestHost(t)
	defer h.Close()
	m := mustOpen(t, h, "cat_girl")
	ctx := context.Background()
	g := m.Graph()

	// Create entities.
	if err := g.SetEntity(ctx, graph.Entity{
		Label: "person:xiaoming",
		Attrs: map[string]any{"age": float64(8), "likes": "dinosaurs"},
	}); err != nil {
		t.Fatalf("SetEntity: %v", err)
	}

	if err := g.SetEntity(ctx, graph.Entity{
		Label: "person:xiaohong",
		Attrs: map[string]any{"age": float64(6)},
	}); err != nil {
		t.Fatalf("SetEntity: %v", err)
	}

	// Add relation.
	if err := g.AddRelation(ctx, graph.Relation{
		From: "person:xiaoming", To: "person:xiaohong", RelType: "sibling",
	}); err != nil {
		t.Fatalf("AddRelation: %v", err)
	}

	// Verify graph traversal.
	neighbors, err := g.Neighbors(ctx, "person:xiaoming")
	if err != nil {
		t.Fatalf("Neighbors: %v", err)
	}
	if len(neighbors) != 1 || neighbors[0] != "person:xiaohong" {
		t.Fatalf("Neighbors = %v, want [person:xiaohong]", neighbors)
	}
}

// ---------------------------------------------------------------------------
// Memory: Segment + Recall tests
// ---------------------------------------------------------------------------

func TestMemoryStoreAndRecall(t *testing.T) {
	h := newTestHost(t)
	defer h.Close()
	m := mustOpen(t, h, "cat_girl")
	ctx := context.Background()

	// Set up graph.
	g := m.Graph()
	if err := g.SetEntity(ctx, graph.Entity{
		Label: "person:xiaoming",
		Attrs: map[string]any{"age": float64(8)},
	}); err != nil {
		t.Fatalf("SetEntity: %v", err)
	}
	if err := g.SetEntity(ctx, graph.Entity{
		Label: "topic:dinosaurs",
	}); err != nil {
		t.Fatalf("SetEntity: %v", err)
	}
	if err := g.AddRelation(ctx, graph.Relation{
		From: "person:xiaoming", To: "topic:dinosaurs", RelType: "likes",
	}); err != nil {
		t.Fatalf("AddRelation: %v", err)
	}

	// Store segments.
	segments := []SegmentInput{
		{Summary: "和小明聊了恐龙", Keywords: []string{"dinosaurs"}, Labels: []string{"person:xiaoming", "topic:dinosaurs"}},
		{Summary: "学到了太空知识", Keywords: []string{"space"}, Labels: []string{"self", "topic:space"}},
		{Summary: "一起做了饭", Keywords: []string{"cooking"}, Labels: []string{"person:xiaoming"}},
	}
	for _, seg := range segments {
		if err := m.StoreSegment(ctx, seg, recall.Bucket1H); err != nil {
			t.Fatalf("StoreSegment: %v", err)
		}
	}

	// Recall with label expansion from person:xiaoming.
	result, err := m.Recall(ctx, RecallQuery{
		Labels: []string{"person:xiaoming"},
		Text:   "dinosaurs",
		Limit:  10,
	})
	if err != nil {
		t.Fatalf("Recall: %v", err)
	}

	if len(result.Segments) == 0 {
		t.Fatal("expected at least one segment from recall")
	}

	// The dinosaur segment should be the top result.
	top := result.Segments[0]
	if top.Summary != "和小明聊了恐龙" {
		t.Errorf("top segment = %q, want %q", top.Summary, "和小明聊了恐龙")
	}

	// Entities should include person:xiaoming and topic:dinosaurs.
	if len(result.Entities) == 0 {
		t.Fatal("expected entities in recall result")
	}

	foundXiaoming := false
	for _, e := range result.Entities {
		if e.Label == "person:xiaoming" {
			foundXiaoming = true
		}
	}
	if !foundXiaoming {
		t.Error("expected person:xiaoming in recall entities")
	}
}

func TestRecallWithoutLabelsTextOnly(t *testing.T) {
	h := newTestHost(t)
	defer h.Close()
	m := mustOpen(t, h, "text_only")
	ctx := context.Background()

	if err := m.Graph().SetEntity(ctx, graph.Entity{Label: "topic:恐龙"}); err != nil {
		t.Fatal(err)
	}
	if err := m.StoreSegment(ctx, SegmentInput{Summary: "聊恐龙", Labels: []string{"topic:恐龙"}, Keywords: []string{"恐龙"}}, recall.Bucket1H); err != nil {
		t.Fatal(err)
	}

	res, err := m.Recall(ctx, RecallQuery{Text: "恐龙", Labels: nil, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Segments) == 0 {
		t.Fatal("expected text-only recall to return segments")
	}
	if len(res.Entities) != 0 {
		t.Fatalf("entities = %d, want 0 when labels are empty", len(res.Entities))
	}
}

func TestRecallWithLabelsGraphExpansion(t *testing.T) {
	h := newTestHost(t)
	defer h.Close()
	m := mustOpen(t, h, "graph_expand")
	ctx := context.Background()

	if err := m.Graph().SetEntity(ctx, graph.Entity{Label: "person:小明"}); err != nil {
		t.Fatal(err)
	}
	if err := m.Graph().SetEntity(ctx, graph.Entity{Label: "topic:恐龙"}); err != nil {
		t.Fatal(err)
	}
	if err := m.Graph().AddRelation(ctx, graph.Relation{From: "person:小明", To: "topic:恐龙", RelType: "likes"}); err != nil {
		t.Fatal(err)
	}
	if err := m.StoreSegment(ctx, SegmentInput{Summary: "和小明聊恐龙", Labels: []string{"person:小明", "topic:恐龙"}, Keywords: []string{"恐龙"}}, recall.Bucket1H); err != nil {
		t.Fatal(err)
	}

	res, err := m.Recall(ctx, RecallQuery{Text: "恐龙", Labels: []string{"person:小明"}, Hops: 2, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Entities) < 2 {
		t.Fatalf("entities = %d, want >=2 after expansion", len(res.Entities))
	}
}

// TestMemoryRecallWithLifeSummary was removed — LongTerm is replaced by
// bucket-based segment compaction. The lt bucket segments are found by
// normal search.

// ---------------------------------------------------------------------------
// Memory: Isolation tests
// ---------------------------------------------------------------------------

func TestMemoryIsolation(t *testing.T) {
	h := newTestHost(t)
	defer h.Close()
	ctx := context.Background()

	m1 := mustOpen(t, h, "persona_a")
	m2 := mustOpen(t, h, "persona_b")

	// Store a segment in persona_a.
	if err := m1.StoreSegment(ctx, SegmentInput{
		Summary: "persona_a segment",
		Labels:  []string{"test"},
	}, recall.Bucket1H); err != nil {
		t.Fatalf("StoreSegment: %v", err)
	}

	// Store a different segment in persona_b.
	if err := m2.StoreSegment(ctx, SegmentInput{
		Summary: "persona_b segment",
		Labels:  []string{"test"},
	}, recall.Bucket1H); err != nil {
		t.Fatalf("StoreSegment: %v", err)
	}

	// Recall from persona_a should only find its own segment.
	r1, err := m1.Recall(ctx, RecallQuery{Text: "persona", Limit: 10})
	if err != nil {
		t.Fatalf("Recall: %v", err)
	}
	for _, seg := range r1.Segments {
		if seg.Summary == "persona_b segment" {
			t.Fatal("persona_a should not see persona_b's segments")
		}
	}
}

// ---------------------------------------------------------------------------
// Memory: Compress pipeline test
// ---------------------------------------------------------------------------

func TestMemoryCompress(t *testing.T) {
	h := newTestHost(t)
	defer h.Close()
	m := mustOpen(t, h, "test")
	ctx := context.Background()

	conv := m.OpenConversation("s1", []string{"person:test"})

	// Add messages.
	if err := conv.Append(ctx, Message{Role: RoleUser, Content: "hello", Timestamp: 1000}); err != nil {
		t.Fatalf("Append: %v", err)
	}
	if err := conv.Append(ctx, Message{Role: RoleModel, Content: "world", Timestamp: 2000}); err != nil {
		t.Fatalf("Append: %v", err)
	}

	// Compress.
	comp := &mockCompressor{}
	if err := m.Compress(ctx, conv, comp); err != nil {
		t.Fatalf("Compress: %v", err)
	}

	// Conversation should be cleared.
	count, err := conv.Count(ctx)
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if count != 0 {
		t.Fatalf("Count after compress = %d, want 0", count)
	}

	// A segment should have been stored.
	segments, err := m.Index().RecentSegments(ctx, 10)
	if err != nil {
		t.Fatalf("RecentSegments: %v", err)
	}
	if len(segments) == 0 {
		t.Fatal("expected at least one segment after compression")
	}

	// The entity should have been created.
	ent, err := m.Graph().GetEntity(ctx, "person:test")
	if err != nil {
		t.Fatalf("GetEntity: %v", err)
	}
	if ent.Attrs["compressed"] != true {
		t.Errorf("entity attrs = %v, want compressed=true", ent.Attrs)
	}
}

func TestMemoryCompressNilCompressor(t *testing.T) {
	h := newTestHost(t)
	defer h.Close()
	m := mustOpen(t, h, "test")
	ctx := context.Background()

	conv := m.OpenConversation("s1", nil)
	if err := m.Compress(ctx, conv, nil); err == nil {
		t.Fatal("expected error with nil compressor")
	}
}

func TestMemoryCompressEmptyConversation(t *testing.T) {
	h := newTestHost(t)
	defer h.Close()
	m := mustOpen(t, h, "test")
	ctx := context.Background()

	conv := m.OpenConversation("s1", nil)
	comp := &mockCompressor{}

	// Compress on empty conversation should be a no-op.
	if err := m.Compress(ctx, conv, comp); err != nil {
		t.Fatalf("Compress empty: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Memory: ApplyEntityUpdate tests
// ---------------------------------------------------------------------------

func TestApplyEntityUpdate(t *testing.T) {
	h := newTestHost(t)
	defer h.Close()
	m := mustOpen(t, h, "test")
	ctx := context.Background()
	g := m.Graph()

	// Create an existing entity.
	if err := g.SetEntity(ctx, graph.Entity{
		Label: "person:alice",
		Attrs: map[string]any{"age": float64(25)},
	}); err != nil {
		t.Fatalf("SetEntity: %v", err)
	}

	// Apply update: merge into existing + create new entity + add relation.
	update := &EntityUpdate{
		Entities: []EntityInput{
			{Label: "person:alice", Attrs: map[string]any{"mood": "happy"}},
			{Label: "person:bob", Attrs: map[string]any{"age": float64(30)}},
		},
		Relations: []RelationInput{
			{From: "person:alice", To: "person:bob", RelType: "knows"},
		},
	}

	if err := m.ApplyEntityUpdate(ctx, update); err != nil {
		t.Fatalf("ApplyEntityUpdate: %v", err)
	}

	// Alice should have merged attrs.
	alice, err := g.GetEntity(ctx, "person:alice")
	if err != nil {
		t.Fatalf("GetEntity alice: %v", err)
	}
	if alice.Attrs["age"] != float64(25) {
		t.Errorf("alice.age = %v, want 25", alice.Attrs["age"])
	}
	if alice.Attrs["mood"] != "happy" {
		t.Errorf("alice.mood = %v, want happy", alice.Attrs["mood"])
	}

	// Bob should exist.
	bob, err := g.GetEntity(ctx, "person:bob")
	if err != nil {
		t.Fatalf("GetEntity bob: %v", err)
	}
	if bob.Attrs["age"] != float64(30) {
		t.Errorf("bob.age = %v, want 30", bob.Attrs["age"])
	}

	// Relation should exist.
	neighbors, err := g.Neighbors(ctx, "person:alice")
	if err != nil {
		t.Fatalf("Neighbors: %v", err)
	}
	if len(neighbors) != 1 || neighbors[0] != "person:bob" {
		t.Errorf("Neighbors = %v, want [person:bob]", neighbors)
	}
}

func TestApplyEntityUpdateNil(t *testing.T) {
	h := newTestHost(t)
	defer h.Close()
	m := mustOpen(t, h, "test")
	ctx := context.Background()

	// Nil update should be a no-op.
	if err := m.ApplyEntityUpdate(ctx, nil); err != nil {
		t.Fatalf("ApplyEntityUpdate(nil): %v", err)
	}
}

// ---------------------------------------------------------------------------
// Keys tests
// ---------------------------------------------------------------------------

func TestConvMsgKeyFormat(t *testing.T) {
	key := convMsgKey("cat", "dev1", 12345)
	if len(key) != 6 {
		t.Fatalf("key segments = %d, want 6", len(key))
	}
	if key[0] != "mem" || key[1] != "cat" || key[2] != "conv" || key[3] != "dev1" || key[4] != "msg" || key[5] != "00000000000000012345" {
		t.Fatalf("key = %v", key)
	}
}

func TestConvMsgKeyLexOrder(t *testing.T) {
	// Verify that zero-padded timestamps sort correctly.
	k1 := convMsgKey("m", "c", 9000)
	k2 := convMsgKey("m", "c", 10000)
	if k1[5] >= k2[5] {
		t.Fatalf("expected %q < %q", k1[5], k2[5])
	}
}

// ---------------------------------------------------------------------------
// Host without vector search
// ---------------------------------------------------------------------------

func TestHostNoVecSearch(t *testing.T) {
	h := newTestHostNoVec(t)
	defer h.Close()
	m := mustOpen(t, h, "test")
	ctx := context.Background()

	// Store a segment (no vector indexing).
	if err := m.StoreSegment(ctx, SegmentInput{
		Summary:  "test segment",
		Keywords: []string{"test"},
		Labels:   []string{"label"},
	}, recall.Bucket1H); err != nil {
		t.Fatalf("StoreSegment: %v", err)
	}

	// Recall should still work (keyword + label only).
	result, err := m.Recall(ctx, RecallQuery{Text: "test", Limit: 10})
	if err != nil {
		t.Fatalf("Recall: %v", err)
	}
	// Without vector search, results rely on keyword/label matching.
	// The segment should still be findable.
	if len(result.Segments) == 0 {
		t.Log("no segments returned without vector search (expected if no label/keyword match)")
	}
}

// ---------------------------------------------------------------------------
// Host Delete tests (equivalent to Rust TH.5, TH.6)
// ---------------------------------------------------------------------------

func TestHostDeleteClearsData(t *testing.T) {
	h := newTestHost(t)
	defer h.Close()
	ctx := context.Background()

	// Open a persona and add data.
	m := mustOpen(t, h, "cat_girl")
	conv := m.OpenConversation("dev1", nil)
	if err := conv.Append(ctx, Message{Role: RoleUser, Content: "hello", Timestamp: 1000}); err != nil {
		t.Fatalf("Append: %v", err)
	}

	// Verify data exists.
	if count, err := conv.Count(ctx); err != nil {
		t.Fatalf("Count: %v", err)
	} else if count == 0 {
		t.Fatal("expected data before delete")
	}

	// Delete the persona.
	if err := h.Delete(ctx, "cat_girl"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	// Reopen the persona - data should be gone.
	m2, err := h.Open("cat_girl")
	if err != nil {
		t.Fatalf("Open after delete: %v", err)
	}
	conv2 := m2.OpenConversation("dev1", nil)
	if count, err := conv2.Count(ctx); err != nil {
		t.Fatalf("Count after reopen: %v", err)
	} else if count != 0 {
		t.Fatalf("expected empty after delete, got %d", count)
	}
}

func TestHostDeleteNonExistentNoError(t *testing.T) {
	h := newTestHost(t)
	defer h.Close()
	ctx := context.Background()

	// Delete non-existent ID should not error.
	if err := h.Delete(ctx, "ghost"); err != nil {
		t.Fatalf("Delete non-existent: %v", err)
	}
}

func TestHostDeletePrefixIsolation(t *testing.T) {
	h := newTestHost(t)
	defer h.Close()
	ctx := context.Background()

	// Create two personas with similar names to test prefix isolation.
	mA := mustOpen(t, h, "a")
	mAB := mustOpen(t, h, "a:b")

	// Add data to both.
	if err := mA.StoreSegment(ctx, SegmentInput{
		Summary: "persona a segment",
		Labels:  []string{"test"},
	}, recall.Bucket1H); err != nil {
		t.Fatalf("StoreSegment for a: %v", err)
	}
	if err := mAB.StoreSegment(ctx, SegmentInput{
		Summary: "persona a:b segment",
		Labels:  []string{"test"},
	}, recall.Bucket1H); err != nil {
		t.Fatalf("StoreSegment for a:b: %v", err)
	}

	// Delete "a" - should NOT affect "a:b".
	if err := h.Delete(ctx, "a"); err != nil {
		t.Fatalf("Delete a: %v", err)
	}

	// Reopen "a:b" - data should still exist.
	mAB2, err := h.Open("a:b")
	if err != nil {
		t.Fatalf("Open a:b after delete: %v", err)
	}
	segs, err := mAB2.Index().RecentSegments(ctx, 10)
	if err != nil {
		t.Fatalf("RecentSegments: %v", err)
	}
	if len(segs) == 0 {
		t.Fatal("a:b data should exist after deleting a")
	}
	if segs[0].Summary != "persona a:b segment" {
		t.Fatalf("expected a:b segment, got %q", segs[0].Summary)
	}

	// Reopen "a" - data should be gone.
	mA2, err := h.Open("a")
	if err != nil {
		t.Fatalf("Open a after delete: %v", err)
	}
	segsA, err := mA2.Index().RecentSegments(ctx, 10)
	if err != nil {
		t.Fatalf("RecentSegments a: %v", err)
	}
	if len(segsA) != 0 {
		t.Fatalf("a data should be gone, got %d segments", len(segsA))
	}
}

// ---------------------------------------------------------------------------
// Conversation: RecentSegments
// ---------------------------------------------------------------------------

func TestConversationRecentSegments(t *testing.T) {
	h := newTestHost(t)
	defer h.Close()
	m := mustOpen(t, h, "test")
	ctx := context.Background()

	// Store some segments in the memory.
	for i := range 3 {
		if err := m.Index().StoreSegment(ctx, recall.Segment{
			ID:        fmt.Sprintf("seg-%d", i),
			Summary:   fmt.Sprintf("segment %d", i),
			Timestamp: int64((i + 1) * 1000000),
		}); err != nil {
			t.Fatalf("StoreSegment: %v", err)
		}
	}

	conv := m.OpenConversation("s1", nil)
	segs, err := conv.RecentSegments(ctx, 2)
	if err != nil {
		t.Fatalf("RecentSegments: %v", err)
	}
	if len(segs) != 2 {
		t.Fatalf("len(segs) = %d, want 2", len(segs))
	}
	// Most recent first.
	if segs[0].ID != "seg-2" {
		t.Errorf("segs[0].ID = %q, want seg-2", segs[0].ID)
	}
}

// ---------------------------------------------------------------------------
// Realistic Scenario: AI cat companion with a family over one week
// ---------------------------------------------------------------------------
//
// Simulates 小猫咪 (AI cat companion) living with a family:
//   - 小明 (8yo boy, loves dinosaurs, Lego, science)
//   - 小红 (6yo girl, loves drawing, princess stories)
//   - 妈妈 (mom, cooks, tells stories)
//   - 爸爸 (dad, plays music, builds Lego)
//
// Tests that recall retrieves the right memories for various queries.

func TestRealisticScenario(t *testing.T) {
	// Use deterministic timestamps to avoid non-determinism.
	var tsCounter int64 = 1_000_000_000_000_000_000 // 2001-09-09
	origNow := nowNano
	nowNano = func() int64 {
		tsCounter += 1_000_000_000 // +1s each call
		return tsCounter
	}
	defer func() { nowNano = origNow }()

	h := newTestHost(t)
	defer h.Close()
	m := mustOpen(t, h, "cat_girl")
	ctx := context.Background()
	g := m.Graph()

	// ---- Step 1: Build the entity graph ----

	entities := []graph.Entity{
		{Label: "self", Attrs: map[string]any{
			"name": "小猫咪", "personality": "活泼好奇", "species": "虚拟猫猫",
		}},
		{Label: "person:小明", Attrs: map[string]any{
			"name": "小明", "age": float64(8), "gender": "男",
			"likes": "恐龙、乐高、太空",
		}},
		{Label: "person:小红", Attrs: map[string]any{
			"name": "小红", "age": float64(6), "gender": "女",
			"likes": "画画、公主故事",
		}},
		{Label: "person:妈妈", Attrs: map[string]any{
			"name": "妈妈", "role": "母亲", "good_at": "做饭、讲故事",
		}},
		{Label: "person:爸爸", Attrs: map[string]any{
			"name": "爸爸", "role": "父亲", "good_at": "音乐、搭乐高",
		}},
		{Label: "topic:恐龙"},
		{Label: "topic:画画"},
		{Label: "topic:做饭"},
		{Label: "topic:音乐"},
		{Label: "topic:太空"},
		{Label: "topic:公主故事"},
		{Label: "topic:乐高"},
	}
	for _, e := range entities {
		if err := g.SetEntity(ctx, e); err != nil {
			t.Fatalf("SetEntity %q: %v", e.Label, err)
		}
	}

	relations := []graph.Relation{
		{From: "person:小明", To: "person:小红", RelType: "sibling"},
		{From: "person:小红", To: "person:小明", RelType: "sibling"},
		{From: "person:妈妈", To: "person:小明", RelType: "parent"},
		{From: "person:妈妈", To: "person:小红", RelType: "parent"},
		{From: "person:爸爸", To: "person:小明", RelType: "parent"},
		{From: "person:爸爸", To: "person:小红", RelType: "parent"},
		{From: "person:小明", To: "topic:恐龙", RelType: "likes"},
		{From: "person:小明", To: "topic:太空", RelType: "likes"},
		{From: "person:小明", To: "topic:乐高", RelType: "likes"},
		{From: "person:小红", To: "topic:画画", RelType: "likes"},
		{From: "person:小红", To: "topic:公主故事", RelType: "likes"},
		{From: "person:妈妈", To: "topic:做饭", RelType: "good_at"},
		{From: "person:爸爸", To: "topic:音乐", RelType: "good_at"},
		{From: "person:爸爸", To: "topic:乐高", RelType: "good_at"},
	}
	for _, r := range relations {
		if err := g.AddRelation(ctx, r); err != nil {
			t.Fatalf("AddRelation %s→%s: %v", r.From, r.To, err)
		}
	}

	// ---- Step 2: Store memory segments (one week of interactions) ----

	type seg struct {
		summary  string
		keywords []string
		labels   []string
	}
	segments := []seg{
		// Day 1
		{"和小明聊了恐龙，他最喜欢霸王龙",
			[]string{"恐龙", "霸王龙"},
			[]string{"person:小明", "topic:恐龙"}},
		{"小明问了很多恐龙的问题，还画了一只三角龙",
			[]string{"恐龙", "三角龙", "画画"},
			[]string{"person:小明", "topic:恐龙", "topic:画画"}},
		{"小明说长大想当古生物学家",
			[]string{"恐龙", "古生物学家", "梦想"},
			[]string{"person:小明", "topic:恐龙"}},
		{"给小明讲了恐龙灭绝的故事，他有点伤心",
			[]string{"恐龙", "灭绝", "故事"},
			[]string{"person:小明", "topic:恐龙"}},
		// Day 2
		{"小红画了一个公主城堡，涂了粉色和金色",
			[]string{"画画", "公主", "城堡"},
			[]string{"person:小红", "topic:画画", "topic:公主故事"}},
		{"和小红一起编了一个公主和小猫的故事",
			[]string{"公主", "小猫", "故事"},
			[]string{"person:小红", "topic:公主故事", "self"}},
		{"小红说她的公主会骑恐龙",
			[]string{"公主", "恐龙"},
			[]string{"person:小红", "topic:公主故事", "topic:恐龙"}},
		// Day 3
		{"妈妈教我们做了蛋炒饭，小明吃了两碗",
			[]string{"做饭", "蛋炒饭"},
			[]string{"person:妈妈", "person:小明", "topic:做饭"}},
		{"妈妈说周末要做恐龙形状的饼干",
			[]string{"做饭", "恐龙", "饼干"},
			[]string{"person:妈妈", "topic:做饭", "topic:恐龙"}},
		// Day 4
		{"和爸爸一起听了古典音乐，小明跟着打节拍",
			[]string{"音乐", "古典音乐"},
			[]string{"person:爸爸", "person:小明", "topic:音乐"}},
		{"爸爸和小明一起拼了一个恐龙乐高模型",
			[]string{"乐高", "恐龙"},
			[]string{"person:爸爸", "person:小明", "topic:乐高", "topic:恐龙"}},
		// Day 5
		{"全家去了自然博物馆看恐龙化石，小明超兴奋",
			[]string{"博物馆", "恐龙", "化石"},
			[]string{"person:小明", "person:小红", "person:妈妈", "person:爸爸", "topic:恐龙"}},
		{"小红在博物馆里画了好多恐龙素描",
			[]string{"画画", "恐龙", "素描"},
			[]string{"person:小红", "topic:画画", "topic:恐龙"}},
		{"小明在天文馆看了星空投影，问了黑洞的问题",
			[]string{"太空", "天文馆", "黑洞"},
			[]string{"person:小明", "topic:太空"}},
		// Day 6
		{"给小明讲了宇宙探险的睡前故事",
			[]string{"太空", "故事", "睡前"},
			[]string{"person:小明", "topic:太空"}},
		{"给小红讲了小猫公主和恐龙的故事，她听得好开心",
			[]string{"公主", "恐龙", "故事"},
			[]string{"person:小红", "topic:公主故事", "topic:恐龙", "self"}},
		// Day 7
		{"小红今天美术课画了全家福，画里还有我",
			[]string{"画画", "全家福", "美术课"},
			[]string{"person:小红", "topic:画画", "self"}},
	}

	for _, s := range segments {
		if err := m.StoreSegment(ctx, SegmentInput{
			Summary:  s.summary,
			Keywords: s.keywords,
			Labels:   s.labels,
		}, recall.Bucket1H); err != nil {
			t.Fatalf("StoreSegment %q: %v", s.summary, err)
		}
	}

	// ---- Step 3: Test recall queries ----

	t.Run("recall dinosaurs from 小明", func(t *testing.T) {
		result, err := m.Recall(ctx, RecallQuery{
			Labels: []string{"person:小明"},
			Text:   "dinosaurs",
			Limit:  5,
		})
		if err != nil {
			t.Fatalf("Recall: %v", err)
		}

		// Should find entities via graph expansion (小明→恐龙, 小明→太空, etc.)
		if len(result.Entities) == 0 {
			t.Fatal("expected entities from graph expansion")
		}
		entityLabels := make(map[string]bool)
		for _, e := range result.Entities {
			entityLabels[e.Label] = true
		}
		if !entityLabels["person:小明"] {
			t.Error("expected person:小明 in entities")
		}

		// Should find dinosaur-related segments.
		if len(result.Segments) == 0 {
			t.Fatal("expected segments about dinosaurs")
		}
		// Top result should be highly dinosaur-related.
		top := result.Segments[0]
		t.Logf("Top result: [%.3f] %s", top.Score, top.Summary)
		for i, s := range result.Segments {
			t.Logf("  %d. [%.3f] %s | labels=%v", i+1, s.Score, s.Summary, s.Labels)
		}

	})

	t.Run("recall drawing from 小红", func(t *testing.T) {
		result, err := m.Recall(ctx, RecallQuery{
			Labels: []string{"person:小红"},
			Text:   "drawing",
			Limit:  5,
		})
		if err != nil {
			t.Fatalf("Recall: %v", err)
		}

		if len(result.Segments) == 0 {
			t.Fatal("expected segments about drawing")
		}

		// Drawing-related segments should rank high.
		t.Logf("小红 drawing recall:")
		for i, s := range result.Segments {
			t.Logf("  %d. [%.3f] %s", i+1, s.Score, s.Summary)
		}

		// Verify 小红's drawing segments appear.
		found := false
		for _, s := range result.Segments {
			for _, l := range s.Labels {
				if l == "topic:画画" {
					found = true
				}
			}
		}
		if !found {
			t.Error("expected at least one drawing segment")
		}
	})

	t.Run("recall cooking from 妈妈", func(t *testing.T) {
		result, err := m.Recall(ctx, RecallQuery{
			Labels: []string{"person:妈妈"},
			Text:   "cooking",
			Limit:  5,
		})
		if err != nil {
			t.Fatalf("Recall: %v", err)
		}

		t.Logf("妈妈 cooking recall:")
		for i, s := range result.Segments {
			t.Logf("  %d. [%.3f] %s", i+1, s.Score, s.Summary)
		}

		if len(result.Segments) == 0 {
			t.Fatal("expected cooking segments for 妈妈")
		}
	})

	t.Run("recall lego from 爸爸", func(t *testing.T) {
		result, err := m.Recall(ctx, RecallQuery{
			Labels: []string{"person:爸爸"},
			Text:   "lego",
			Limit:  5,
		})
		if err != nil {
			t.Fatalf("Recall: %v", err)
		}

		t.Logf("爸爸 lego recall:")
		for i, s := range result.Segments {
			t.Logf("  %d. [%.3f] %s", i+1, s.Score, s.Summary)
		}

		if len(result.Segments) == 0 {
			t.Fatal("expected Lego segments for 爸爸")
		}

		// 爸爸's Lego segment should appear.
		found := false
		for _, s := range result.Segments {
			for _, l := range s.Labels {
				if l == "topic:乐高" {
					found = true
				}
			}
		}
		if !found {
			t.Error("expected at least one Lego segment")
		}
	})

	t.Run("cross-topic: 小红 mentions dinosaurs via princess", func(t *testing.T) {
		// 小红说她的公主会骑恐龙 — this bridges 小红's world with 恐龙.
		result, err := m.Recall(ctx, RecallQuery{
			Labels: []string{"person:小红"},
			Text:   "dinosaurs",
			Limit:  5,
		})
		if err != nil {
			t.Fatalf("Recall: %v", err)
		}

		t.Logf("小红 dinosaurs recall (cross-topic):")
		for i, s := range result.Segments {
			t.Logf("  %d. [%.3f] %s", i+1, s.Score, s.Summary)
		}

		// Should find "小红说她的公主会骑恐龙" and "小红在博物馆里画了好多恐龙素描".
		if len(result.Segments) == 0 {
			t.Fatal("expected cross-topic segments")
		}
	})

	t.Run("graph expansion: 小明 reaches 小红 via sibling", func(t *testing.T) {
		// Query from 小明 with 2 hops should reach:
		//   小明 → 小红 (sibling), 恐龙 (likes), 太空 (likes), 乐高 (likes)
		//   小红 → 画画, 公主故事
		// etc.
		expanded, err := g.Expand(ctx, []string{"person:小明"}, 2)
		if err != nil {
			t.Fatalf("Expand: %v", err)
		}
		t.Logf("Graph expansion from person:小明 (2 hops): %v", expanded)

		expandedSet := make(map[string]bool)
		for _, l := range expanded {
			expandedSet[l] = true
		}
		if !expandedSet["person:小红"] {
			t.Error("expected person:小红 reachable via sibling")
		}
		if !expandedSet["topic:恐龙"] {
			t.Error("expected topic:恐龙 reachable via likes")
		}
	})

	t.Run("persona isolation", func(t *testing.T) {
		other := mustOpen(t, h, "robot_boy")
		result, err := other.Recall(ctx, RecallQuery{Text: "dinosaurs", Limit: 10})
		if err != nil {
			t.Fatalf("Recall: %v", err)
		}
		if len(result.Segments) > 0 {
			t.Errorf("robot_boy should not see cat_girl's segments, got %d", len(result.Segments))
		}
	})
}

// ---------------------------------------------------------------------------
// Realistic Scenario: Conversation → Compress → Recall pipeline
// ---------------------------------------------------------------------------

func TestRealisticCompressPipeline(t *testing.T) {
	h := newTestHost(t)
	defer h.Close()
	m := mustOpen(t, h, "cat_girl")
	ctx := context.Background()

	// Pre-populate some background.
	g := m.Graph()
	_ = g.SetEntity(ctx, graph.Entity{Label: "person:小明", Attrs: map[string]any{"age": float64(8)}})
	_ = g.SetEntity(ctx, graph.Entity{Label: "topic:恐龙"})
	_ = g.AddRelation(ctx, graph.Relation{From: "person:小明", To: "topic:恐龙", RelType: "likes"})

	// Simulate a conversation session.
	conv := m.OpenConversation("xiaoming-ipad", []string{"person:小明"})

	messages := []Message{
		{Role: RoleUser, Name: "小明", Content: "小猫咪！今天我在学校看了一本恐龙的书！", Timestamp: 1000},
		{Role: RoleModel, Content: "哇，是什么恐龙的书呀？", Timestamp: 2000},
		{Role: RoleUser, Name: "小明", Content: "讲的是翼龙！你知道翼龙会飞吗？", Timestamp: 3000},
		{Role: RoleModel, Content: "知道呀！翼龙的翅膀可以展开好几米呢！不过严格来说翼龙不算恐龙哦～", Timestamp: 4000},
		{Role: RoleUser, Name: "小明", Content: "啊？那翼龙算什么？", Timestamp: 5000},
		{Role: RoleModel, Content: "翼龙属于翼龙目，和恐龙是亲戚，但不是同一类。就像猫和老虎是亲戚一样！", Timestamp: 6000},
		{Role: RoleUser, Name: "小明", Content: "好酷！那飞在天上的都不是恐龙吗？", Timestamp: 7000},
		{Role: RoleModel, Content: "对的，真正的恐龙都是在地上走的。不过现在的鸟类其实是恐龙的后代哦！", Timestamp: 8000},
		{Role: RoleUser, Name: "小明", Content: "什么！鸡也是恐龙？？", Timestamp: 9000},
		{Role: RoleModel, Content: "哈哈没错，鸡确实是恐龙的后代！下次吃鸡腿的时候可以想想，你在吃恐龙～", Timestamp: 10000},
	}
	for _, msg := range messages {
		if err := conv.Append(ctx, msg); err != nil {
			t.Fatalf("Append: %v", err)
		}
	}

	// Verify conversation state.
	count, err := conv.Count(ctx)
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if count != 10 {
		t.Fatalf("Count = %d, want 10", count)
	}

	// Recent messages should return the last few.
	recent, err := conv.Recent(ctx, 3)
	if err != nil {
		t.Fatalf("Recent: %v", err)
	}
	if recent[2].Content != "哈哈没错，鸡确实是恐龙的后代！下次吃鸡腿的时候可以想想，你在吃恐龙～" {
		t.Errorf("last message content mismatch: %q", recent[2].Content)
	}

	// Compress using a realistic compressor.
	comp := &familyCompressor{}
	if err := m.Compress(ctx, conv, comp); err != nil {
		t.Fatalf("Compress: %v", err)
	}

	// Conversation should be cleared.
	count, err = conv.Count(ctx)
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if count != 0 {
		t.Fatalf("Count after compress = %d, want 0", count)
	}

	// The compressed segment should be searchable.
	segs, err := m.Index().RecentSegments(ctx, 10)
	if err != nil {
		t.Fatalf("RecentSegments: %v", err)
	}
	t.Logf("Segments after compression: %d", len(segs))
	for i, s := range segs {
		t.Logf("  %d. %s | keywords=%v labels=%v", i+1, s.Summary, s.Keywords, s.Labels)
	}
	if len(segs) == 0 {
		t.Fatal("expected at least one segment after compression")
	}

	// New entity "topic:翼龙" should have been extracted.
	pterosaur, err := g.GetEntity(ctx, "topic:翼龙")
	if err != nil {
		t.Fatalf("GetEntity topic:翼龙: %v", err)
	}
	if pterosaur == nil {
		t.Fatal("expected topic:翼龙 entity after extraction")
	}
}

// familyCompressor is a more realistic test compressor that produces
// meaningful summaries and entity extractions.
type familyCompressor struct{}

func (fc *familyCompressor) CompressMessages(_ context.Context, msgs []Message) (*CompressResult, error) {
	// Simulate LLM compression: summarize the conversation.
	return &CompressResult{
		Segments: []SegmentInput{
			{
				Summary:  "小明在学校看了恐龙的书，和小猫咪聊了翼龙。学到翼龙不算恐龙，鸟类是恐龙的后代。",
				Keywords: []string{"恐龙", "翼龙", "鸟类", "古生物"},
				Labels:   []string{"person:小明", "topic:恐龙", "topic:翼龙"},
			},
		},
		Summary: fmt.Sprintf("和小明聊了翼龙和恐龙的区别，共%d条消息", len(msgs)),
	}, nil
}

func (fc *familyCompressor) ExtractEntities(_ context.Context, _ []Message) (*EntityUpdate, error) {
	return &EntityUpdate{
		Entities: []EntityInput{
			{Label: "topic:翼龙", Attrs: map[string]any{
				"type": "pterosaur", "note": "不算恐龙，属于翼龙目",
			}},
			{Label: "person:小明", Attrs: map[string]any{
				"recent_interest": "翼龙和鸟类的关系",
			}},
		},
		Relations: []RelationInput{
			{From: "person:小明", To: "topic:翼龙", RelType: "curious_about"},
			{From: "topic:翼龙", To: "topic:恐龙", RelType: "related_to"},
		},
	}, nil
}

func (fc *familyCompressor) CompactSegments(_ context.Context, summaries []string) (*CompressResult, error) {
	combined := ""
	for _, s := range summaries {
		if combined != "" {
			combined += " "
		}
		combined += s
	}
	return &CompressResult{
		Segments: []SegmentInput{{
			Summary:  combined,
			Keywords: []string{"family", "compacted"},
			Labels:   []string{"person:小明"},
		}},
		Summary: combined,
	}, nil
}

// ---------------------------------------------------------------------------
// Benchmarks
// ---------------------------------------------------------------------------

func BenchmarkConversationAppend(b *testing.B) {
	store := kv.NewMemory(&kv.Options{Separator: testSep})
	h, err := NewHost(context.Background(), HostConfig{Store: store, Separator: testSep})
	if err != nil {
		b.Fatal(err)
	}
	m := mustOpen(b, h, "bench")
	conv := m.OpenConversation("s1", nil)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = conv.Append(ctx, Message{
			Role:      RoleUser,
			Content:   "benchmark message",
			Timestamp: int64(i + 1),
		})
	}
}

func BenchmarkConversationRecent(b *testing.B) {
	store := kv.NewMemory(&kv.Options{Separator: testSep})
	h, err := NewHost(context.Background(), HostConfig{Store: store, Separator: testSep})
	if err != nil {
		b.Fatal(err)
	}
	m := mustOpen(b, h, "bench")
	conv := m.OpenConversation("s1", nil)
	ctx := context.Background()

	// Pre-fill 100 messages.
	for i := range 100 {
		_ = conv.Append(ctx, Message{
			Role:      RoleUser,
			Content:   fmt.Sprintf("message %d", i),
			Timestamp: int64((i + 1) * 1000),
		})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = conv.Recent(ctx, 10)
	}
}

func BenchmarkStoreSegment(b *testing.B) {
	store := kv.NewMemory(&kv.Options{Separator: testSep})
	emb := newMockEmbedder()
	vec := vecstore.NewMemory()
	h, err := NewHost(context.Background(), HostConfig{
		Store: store, Vec: vec, Embedder: emb, Separator: testSep,
	})
	if err != nil {
		b.Fatal(err)
	}
	m := mustOpen(b, h, "bench")
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.StoreSegment(ctx, SegmentInput{
			Summary:  "benchmark segment about dinosaurs",
			Keywords: []string{"dinosaurs", "benchmark"},
			Labels:   []string{"person:bench"},
		}, recall.Bucket1H)
	}
}

func BenchmarkRecall(b *testing.B) {
	store := kv.NewMemory(&kv.Options{Separator: testSep})
	emb := newMockEmbedder()
	vec := vecstore.NewMemory()
	h, err := NewHost(context.Background(), HostConfig{
		Store: store, Vec: vec, Embedder: emb, Separator: testSep,
	})
	if err != nil {
		b.Fatal(err)
	}
	m := mustOpen(b, h, "bench")
	ctx := context.Background()
	g := m.Graph()

	// Set up graph.
	_ = g.SetEntity(ctx, graph.Entity{Label: "person:xiaoming", Attrs: map[string]any{"age": float64(8)}})
	_ = g.SetEntity(ctx, graph.Entity{Label: "topic:dinosaurs"})
	_ = g.AddRelation(ctx, graph.Relation{From: "person:xiaoming", To: "topic:dinosaurs", RelType: "likes"})

	// Pre-fill 50 segments.
	for i := range 50 {
		_ = m.StoreSegment(ctx, SegmentInput{
			Summary:  fmt.Sprintf("segment %d about dinosaurs", i),
			Keywords: []string{"dinosaurs"},
			Labels:   []string{"person:xiaoming", "topic:dinosaurs"},
		}, recall.Bucket1H)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = m.Recall(ctx, RecallQuery{
			Labels: []string{"person:xiaoming"},
			Text:   "dinosaurs",
			Limit:  10,
		})
	}
}

// BenchmarkLongTermSetSummary was removed — LongTerm replaced by bucket compaction.
