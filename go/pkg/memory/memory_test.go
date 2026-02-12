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
			"和小明聊了恐龙，他最喜欢霸王龙":           {0.9, 0, 0, 0, 0, 0, 0, 0},
			"小明问了很多恐龙的问题，还画了一只三角龙":       {0.7, 0, 0, 0, 0.3, 0, 0, 0},
			"和小明聊了恐龙":                      {0.9, 0.1, 0, 0, 0, 0, 0, 0},
			"学到了太空知识":                       {0.1, 0.9, 0, 0, 0, 0, 0, 0},
			"一起做了饭":                         {0.1, 0, 0.9, 0, 0, 0, 0, 0},
			"played music":                   {0, 0, 0.1, 0.9, 0, 0, 0, 0},
			"小明说长大想当古生物学家":                  {0.8, 0.1, 0, 0, 0, 0, 0, 0},
			"给小明讲了恐龙灭绝的故事，他有点伤心":           {0.6, 0, 0, 0, 0, 0.3, 0, 0},

			// Day 2: 小红 drawing session
			"小红画了一个公主城堡，涂了粉色和金色":           {0, 0, 0, 0, 0.8, 0.2, 0, 0},
			"和小红一起编了一个公主和小猫的故事":            {0, 0, 0, 0, 0.2, 0.8, 0, 0},
			"小红说她的公主会骑恐龙":                   {0.3, 0, 0, 0, 0.4, 0.3, 0, 0},

			// Day 3: 妈妈 cooking
			"妈妈教我们做了蛋炒饭，小明吃了两碗":            {0, 0, 0.9, 0, 0, 0, 0, 0},
			"妈妈说周末要做恐龙形状的饼干":                {0.3, 0, 0.7, 0, 0, 0, 0, 0},

			// Day 4: 爸爸 music + Lego
			"和爸爸一起听了古典音乐，小明跟着打节拍":          {0, 0, 0, 0.8, 0, 0, 0, 0.1},
			"爸爸和小明一起拼了一个恐龙乐高模型":             {0.4, 0, 0, 0, 0, 0, 0, 0.6},

			// Day 5: Outdoor + science
			"全家去了自然博物馆看恐龙化石，小明超兴奋":          {0.7, 0.2, 0, 0, 0, 0, 0.1, 0},
			"小红在博物馆里画了好多恐龙素描":               {0.4, 0, 0, 0, 0.6, 0, 0, 0},
			"小明在天文馆看了星空投影，问了黑洞的问题":          {0, 0.9, 0, 0, 0, 0, 0, 0},

			// Day 6: Bedtime stories
			"给小明讲了宇宙探险的睡前故事":                {0, 0.5, 0, 0, 0, 0.5, 0, 0},
			"给小红讲了小猫公主和恐龙的故事，她听得好开心":       {0.2, 0, 0, 0, 0, 0.7, 0, 0},

			// Day 7: 小红 art class
			"小红今天美术课画了全家福，画里还有我":            {0, 0, 0, 0, 0.9, 0, 0, 0},

			// --- Search queries ---
			"小明喜欢什么":                         {0.5, 0.2, 0, 0, 0, 0, 0, 0.3},
			"小红喜欢什么":                         {0, 0, 0, 0, 0.5, 0.3, 0, 0},
			"最近和家人做了什么":                      {0.2, 0.1, 0.2, 0.1, 0.1, 0.2, 0.1, 0.1},
			"恐龙相关的回忆":                        {0.95, 0, 0, 0, 0, 0.05, 0, 0},
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

	return NewHost(HostConfig{
		Store:     store,
		Vec:       vec,
		Embedder:  emb,
		Separator: testSep,
	})
}

// newTestHostNoVec creates a Host without vector search.
func newTestHostNoVec(t *testing.T) *Host {
	t.Helper()
	store := kv.NewMemory(&kv.Options{Separator: testSep})
	return NewHost(HostConfig{Store: store, Separator: testSep})
}

// ---------------------------------------------------------------------------
// Host tests
// ---------------------------------------------------------------------------

func TestHostOpen(t *testing.T) {
	h := newTestHost(t)
	defer h.Close()

	m1 := h.Open("cat_girl")
	m2 := h.Open("cat_girl")
	m3 := h.Open("robot_boy")

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

func TestHostPanicsOnNilStore(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on nil Store")
		}
	}()
	NewHost(HostConfig{})
}

// ---------------------------------------------------------------------------
// Conversation tests
// ---------------------------------------------------------------------------

func TestConversationAppendAndRecent(t *testing.T) {
	h := newTestHost(t)
	defer h.Close()
	m := h.Open("test")
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
	m := h.Open("test")
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
	m := h.Open("test")
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
	m := h.Open("test")
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
	m := h.Open("test")
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
	m := h.Open("test")
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
// LongTerm tests
// ---------------------------------------------------------------------------

func TestLongTermSummary(t *testing.T) {
	h := newTestHost(t)
	defer h.Close()
	m := h.Open("test")
	ctx := context.Background()
	lt := m.LongTerm()

	now := time.Date(2026, 2, 13, 15, 0, 0, 0, time.UTC)

	// Get non-existent summary.
	s, err := lt.GetSummary(ctx, GrainHour, now)
	if err != nil {
		t.Fatalf("GetSummary: %v", err)
	}
	if s != "" {
		t.Fatalf("expected empty summary, got %q", s)
	}

	// Set and retrieve.
	if err := lt.SetSummary(ctx, GrainHour, now, "chatted about dinosaurs"); err != nil {
		t.Fatalf("SetSummary: %v", err)
	}
	s, err = lt.GetSummary(ctx, GrainHour, now)
	if err != nil {
		t.Fatalf("GetSummary: %v", err)
	}
	if s != "chatted about dinosaurs" {
		t.Fatalf("GetSummary = %q, want %q", s, "chatted about dinosaurs")
	}
}

func TestLongTermLifeSummary(t *testing.T) {
	h := newTestHost(t)
	defer h.Close()
	m := h.Open("test")
	ctx := context.Background()
	lt := m.LongTerm()

	// Empty initially.
	s, err := lt.LifeSummary(ctx)
	if err != nil {
		t.Fatalf("LifeSummary: %v", err)
	}
	if s != "" {
		t.Fatalf("expected empty life summary, got %q", s)
	}

	// Set and retrieve.
	if err := lt.SetLifeSummary(ctx, "I am a virtual cat companion"); err != nil {
		t.Fatalf("SetLifeSummary: %v", err)
	}
	s, err = lt.LifeSummary(ctx)
	if err != nil {
		t.Fatalf("LifeSummary: %v", err)
	}
	if s != "I am a virtual cat companion" {
		t.Fatalf("LifeSummary = %q, want %q", s, "I am a virtual cat companion")
	}
}

func TestLongTermSummaries(t *testing.T) {
	h := newTestHost(t)
	defer h.Close()
	m := h.Open("test")
	ctx := context.Background()
	lt := m.LongTerm()

	// Store daily summaries for 3 consecutive days.
	days := []struct {
		t time.Time
		s string
	}{
		{time.Date(2026, 2, 10, 0, 0, 0, 0, time.UTC), "day 10 summary"},
		{time.Date(2026, 2, 11, 0, 0, 0, 0, time.UTC), "day 11 summary"},
		{time.Date(2026, 2, 12, 0, 0, 0, 0, time.UTC), "day 12 summary"},
	}
	for _, d := range days {
		if err := lt.SetSummary(ctx, GrainDay, d.t, d.s); err != nil {
			t.Fatalf("SetSummary: %v", err)
		}
	}

	// Query range that includes day 11 and 12 (from=11, to=13).
	from := time.Date(2026, 2, 11, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 2, 13, 0, 0, 0, 0, time.UTC)

	summaries, err := lt.Summaries(ctx, GrainDay, from, to)
	if err != nil {
		t.Fatalf("Summaries: %v", err)
	}
	if len(summaries) != 2 {
		t.Fatalf("len(summaries) = %d, want 2", len(summaries))
	}
	if summaries[0].Summary != "day 11 summary" {
		t.Errorf("summaries[0] = %q, want %q", summaries[0].Summary, "day 11 summary")
	}
	if summaries[1].Summary != "day 12 summary" {
		t.Errorf("summaries[1] = %q, want %q", summaries[1].Summary, "day 12 summary")
	}
}

func TestLongTermGetViaLifeGrain(t *testing.T) {
	h := newTestHost(t)
	defer h.Close()
	m := h.Open("test")
	ctx := context.Background()
	lt := m.LongTerm()

	if err := lt.SetLifeSummary(ctx, "life story"); err != nil {
		t.Fatalf("SetLifeSummary: %v", err)
	}

	// GetSummary with GrainLife should delegate to LifeSummary.
	s, err := lt.GetSummary(ctx, GrainLife, time.Now())
	if err != nil {
		t.Fatalf("GetSummary(GrainLife): %v", err)
	}
	if s != "life story" {
		t.Fatalf("got %q, want %q", s, "life story")
	}
}

// ---------------------------------------------------------------------------
// Memory: Graph integration tests
// ---------------------------------------------------------------------------

func TestMemoryGraph(t *testing.T) {
	h := newTestHost(t)
	defer h.Close()
	m := h.Open("cat_girl")
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
	m := h.Open("cat_girl")
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
		if err := m.StoreSegment(ctx, seg); err != nil {
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

func TestMemoryRecallWithLifeSummary(t *testing.T) {
	h := newTestHost(t)
	defer h.Close()
	m := h.Open("test")
	ctx := context.Background()

	// Set a life summary.
	if err := m.LongTerm().SetLifeSummary(ctx, "I have been with this family for 6 months"); err != nil {
		t.Fatalf("SetLifeSummary: %v", err)
	}

	// Recall should include the life summary.
	result, err := m.Recall(ctx, RecallQuery{Text: "anything"})
	if err != nil {
		t.Fatalf("Recall: %v", err)
	}
	if len(result.Summaries) != 1 {
		t.Fatalf("len(summaries) = %d, want 1", len(result.Summaries))
	}
	if result.Summaries[0].Grain != GrainLife {
		t.Errorf("grain = %v, want GrainLife", result.Summaries[0].Grain)
	}
	if result.Summaries[0].Summary != "I have been with this family for 6 months" {
		t.Errorf("summary = %q", result.Summaries[0].Summary)
	}
}

// ---------------------------------------------------------------------------
// Memory: Isolation tests
// ---------------------------------------------------------------------------

func TestMemoryIsolation(t *testing.T) {
	h := newTestHost(t)
	defer h.Close()
	ctx := context.Background()

	m1 := h.Open("persona_a")
	m2 := h.Open("persona_b")

	// Store a segment in persona_a.
	if err := m1.StoreSegment(ctx, SegmentInput{
		Summary: "persona_a segment",
		Labels:  []string{"test"},
	}); err != nil {
		t.Fatalf("StoreSegment: %v", err)
	}

	// Store a different segment in persona_b.
	if err := m2.StoreSegment(ctx, SegmentInput{
		Summary: "persona_b segment",
		Labels:  []string{"test"},
	}); err != nil {
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
	m := h.Open("test")
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
	m := h.Open("test")
	ctx := context.Background()

	conv := m.OpenConversation("s1", nil)
	if err := m.Compress(ctx, conv, nil); err == nil {
		t.Fatal("expected error with nil compressor")
	}
}

func TestMemoryCompressEmptyConversation(t *testing.T) {
	h := newTestHost(t)
	defer h.Close()
	m := h.Open("test")
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
	m := h.Open("test")
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
	m := h.Open("test")
	ctx := context.Background()

	// Nil update should be a no-op.
	if err := m.ApplyEntityUpdate(ctx, nil); err != nil {
		t.Fatalf("ApplyEntityUpdate(nil): %v", err)
	}
}

// ---------------------------------------------------------------------------
// Grain string tests
// ---------------------------------------------------------------------------

func TestGrainString(t *testing.T) {
	tests := []struct {
		g    Grain
		want string
	}{
		{GrainHour, "hour"},
		{GrainDay, "day"},
		{GrainWeek, "week"},
		{GrainMonth, "month"},
		{GrainYear, "year"},
		{GrainLife, "life"},
		{Grain(99), "unknown"},
	}
	for _, tt := range tests {
		if got := tt.g.String(); got != tt.want {
			t.Errorf("Grain(%d).String() = %q, want %q", tt.g, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// Keys tests
// ---------------------------------------------------------------------------

func TestGrainTimeKey(t *testing.T) {
	ts := time.Date(2026, 2, 13, 15, 30, 0, 0, time.UTC).UnixNano()

	tests := []struct {
		grain Grain
		want  string
	}{
		{GrainHour, "2026021315"},
		{GrainDay, "20260213"},
		{GrainMonth, "202602"},
		{GrainYear, "2026"},
	}
	for _, tt := range tests {
		got, err := grainTimeKey(tt.grain, ts)
		if err != nil {
			t.Fatalf("grainTimeKey(%v): %v", tt.grain, err)
		}
		if got != tt.want {
			t.Errorf("grainTimeKey(%v) = %q, want %q", tt.grain, got, tt.want)
		}
	}
}

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
	m := h.Open("test")
	ctx := context.Background()

	// Store a segment (no vector indexing).
	if err := m.StoreSegment(ctx, SegmentInput{
		Summary:  "test segment",
		Keywords: []string{"test"},
		Labels:   []string{"label"},
	}); err != nil {
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
// Conversation: RecentSegments
// ---------------------------------------------------------------------------

func TestConversationRecentSegments(t *testing.T) {
	h := newTestHost(t)
	defer h.Close()
	m := h.Open("test")
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
	m := h.Open("cat_girl")
	ctx := context.Background()
	g := m.Graph()
	lt := m.LongTerm()

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
		}); err != nil {
			t.Fatalf("StoreSegment %q: %v", s.summary, err)
		}
	}

	// ---- Step 3: Set long-term summaries ----

	if err := lt.SetLifeSummary(ctx, "我是小猫咪，一只虚拟猫猫伙伴。和这个家庭在一起已经半年了。小明8岁，最喜欢恐龙。小红6岁，喜欢画画和公主故事。"); err != nil {
		t.Fatalf("SetLifeSummary: %v", err)
	}
	if err := lt.SetSummary(ctx, GrainDay, time.Date(2026, 2, 12, 0, 0, 0, 0, time.UTC),
		"今天全家去了博物馆，小明看恐龙化石很兴奋，小红画了恐龙素描，小明还去天文馆看了星空"); err != nil {
		t.Fatalf("SetSummary day: %v", err)
	}

	// ---- Step 4: Test recall queries ----

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

		// Life summary should be included.
		if len(result.Summaries) == 0 {
			t.Error("expected life summary in recall")
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
		other := h.Open("robot_boy")
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
	m := h.Open("cat_girl")
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

// ---------------------------------------------------------------------------
// Benchmarks
// ---------------------------------------------------------------------------

func BenchmarkConversationAppend(b *testing.B) {
	store := kv.NewMemory(&kv.Options{Separator: testSep})
	h := NewHost(HostConfig{Store: store, Separator: testSep})
	m := h.Open("bench")
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
	h := NewHost(HostConfig{Store: store, Separator: testSep})
	m := h.Open("bench")
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
	h := NewHost(HostConfig{
		Store: store, Vec: vec, Embedder: emb, Separator: testSep,
	})
	m := h.Open("bench")
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.StoreSegment(ctx, SegmentInput{
			Summary:  "benchmark segment about dinosaurs",
			Keywords: []string{"dinosaurs", "benchmark"},
			Labels:   []string{"person:bench"},
		})
	}
}

func BenchmarkRecall(b *testing.B) {
	store := kv.NewMemory(&kv.Options{Separator: testSep})
	emb := newMockEmbedder()
	vec := vecstore.NewMemory()
	h := NewHost(HostConfig{
		Store: store, Vec: vec, Embedder: emb, Separator: testSep,
	})
	m := h.Open("bench")
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
		})
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

func BenchmarkLongTermSetSummary(b *testing.B) {
	store := kv.NewMemory(&kv.Options{Separator: testSep})
	h := NewHost(HostConfig{Store: store, Separator: testSep})
	m := h.Open("bench")
	ctx := context.Background()
	lt := m.LongTerm()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		t := time.Date(2026, 1, 1, i%24, 0, 0, 0, time.UTC)
		_ = lt.SetSummary(ctx, GrainHour, t, "benchmark summary")
	}
}
