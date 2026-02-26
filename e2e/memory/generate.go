package main

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/haivivi/giztoy/go/pkg/memory"
	msgpack "github.com/vmihailenco/msgpack/v5"
)

// ---------------------------------------------------------------------------
// YAML types for test case files
// ---------------------------------------------------------------------------

// Meta is written to meta.yaml in each test case directory.
type Meta struct {
	Name    string     `yaml:"name"`
	Desc    string     `yaml:"desc"`
	Persona string     `yaml:"persona"`
	Total   int        `yaml:"total_messages"`
	Convs   int        `yaml:"conversations"`
	Expect  MetaExpect `yaml:"expect"`
}

// MetaExpect holds pass/fail criteria checked after all conversations.
type MetaExpect struct {
	MinEntities     int            `yaml:"min_entities"`
	EntitiesContain []string       `yaml:"entities_contain,omitempty"`
	MinRelations    int            `yaml:"min_relations,omitempty"`
	MinSegments     int            `yaml:"min_segments,omitempty"`
	Recall          []RecallExpect `yaml:"recall,omitempty"`
}

// RecallExpect specifies a recall query and minimum results.
type RecallExpect struct {
	Text       string   `yaml:"text"`
	Labels     []string `yaml:"labels,omitempty"`
	MinResults int      `yaml:"min_results"`
}

// ConvFile is a single conversation file (conv_NNN.yaml).
type ConvFile struct {
	ConvID   string    `yaml:"conv_id"`
	Labels   []string  `yaml:"labels,omitempty"`
	Messages []ConvMsg `yaml:"messages"`
}

// ConvMsg is a single message in a conversation.
type ConvMsg struct {
	Role    string `yaml:"role"`
	Name    string `yaml:"name,omitempty"`
	Content string `yaml:"content"`
}

// ---------------------------------------------------------------------------
// Scenario specs
// ---------------------------------------------------------------------------

type scenario struct {
	Name     string
	Desc     string
	Persona  string
	Total    int // total messages across all conversations
	ConvSize int // messages per conversation
	Personas []genPersona
	Topics   []string
	Places   []string
	Purpose  string
	Expect   MetaExpect
}

type genPersona struct {
	Name  string
	Type  string
	Age   int
	Attrs map[string]string // extra key-value attrs to reveal
}

func buildScenarios() []scenario {
	return []scenario{
		// 4 x 10 messages
		{
			Name: "m01_single_person", Desc: "Single person, basic facts", Persona: "cat_girl",
			Total: 10, ConvSize: 10,
			Personas: []genPersona{{Name: "小明", Type: "person", Age: 8, Attrs: map[string]string{"hobby": "恐龙"}}},
			Topics:   []string{"恐龙", "学校"}, Places: []string{"北京"},
			Purpose: "basic",
			Expect:  MetaExpect{MinEntities: 1, EntitiesContain: []string{"person:小明"}, MinSegments: 1},
		},
		{
			Name: "m02_two_siblings", Desc: "Two siblings, relation discovery", Persona: "cat_girl",
			Total: 10, ConvSize: 10,
			Personas: []genPersona{
				{Name: "小明", Type: "person", Age: 8, Attrs: map[string]string{"hobby": "恐龙"}},
				{Name: "小红", Type: "person", Age: 6, Attrs: map[string]string{"hobby": "画画"}},
			},
			Topics: []string{"恐龙", "画画"}, Purpose: "relation",
			Expect: MetaExpect{MinEntities: 2, EntitiesContain: []string{"person:小明", "person:小红"}, MinRelations: 1, MinSegments: 1},
		},
		{
			Name: "m03_work_chat", Desc: "Office work discussion, English names", Persona: "assistant",
			Total: 10, ConvSize: 10,
			Personas: []genPersona{
				{Name: "Alice", Type: "person", Age: 30, Attrs: map[string]string{"job": "engineer", "company": "TechCorp"}},
			},
			Topics: []string{"project", "deadline", "code review"}, Places: []string{"office"},
			Purpose: "basic",
			Expect:  MetaExpect{MinEntities: 1, EntitiesContain: []string{"person:Alice"}, MinSegments: 1},
		},
		{
			Name: "m04_cooking", Desc: "Mom teaches cooking", Persona: "cat_girl",
			Total: 10, ConvSize: 10,
			Personas: []genPersona{
				{Name: "妈妈", Type: "person", Age: 38, Attrs: map[string]string{"skill": "做饭"}},
				{Name: "小明", Type: "person", Age: 8},
			},
			Topics: []string{"蛋炒饭", "饼干", "做饭"}, Purpose: "multi_speaker",
			Expect: MetaExpect{MinEntities: 2, MinSegments: 1},
		},

		// 3 x 100 messages
		{
			Name: "m05_family_week", Desc: "A week with the family, facts accumulate across 5 sessions", Persona: "cat_girl",
			Total: 100, ConvSize: 20,
			Personas: []genPersona{
				{Name: "小明", Type: "person", Age: 8, Attrs: map[string]string{"hobby": "恐龙", "dream": "古生物学家"}},
				{Name: "小红", Type: "person", Age: 6, Attrs: map[string]string{"hobby": "画画"}},
				{Name: "妈妈", Type: "person", Age: 38, Attrs: map[string]string{"skill": "做饭"}},
				{Name: "爸爸", Type: "person", Age: 40, Attrs: map[string]string{"hobby": "音乐"}},
			},
			Topics: []string{"恐龙", "画画", "做饭", "音乐", "乐高", "故事"}, Places: []string{"博物馆", "学校", "家"},
			Purpose: "accumulation",
			Expect: MetaExpect{
				MinEntities: 4, EntitiesContain: []string{"person:小明", "person:小红"}, MinRelations: 2, MinSegments: 2,
				Recall: []RecallExpect{{Text: "恐龙", Labels: []string{"person:小明"}, MinResults: 1}},
			},
		},
		{
			Name: "m06_topic_drift", Desc: "Topics shift gradually across 5 sessions", Persona: "assistant",
			Total: 100, ConvSize: 20,
			Personas: []genPersona{
				{Name: "小王", Type: "person", Age: 27, Attrs: map[string]string{"job": "设计师"}},
			},
			Topics: []string{"旅行", "美食", "电影", "运动", "摄影", "读书"}, Places: []string{"上海", "成都", "广州", "东京"},
			Purpose: "topic_drift",
			Expect:  MetaExpect{MinEntities: 1, EntitiesContain: []string{"person:小王"}, MinSegments: 2},
		},
		{
			Name: "m07_corrections", Desc: "Facts get corrected and updated across 5 sessions", Persona: "assistant",
			Total: 100, ConvSize: 20,
			Personas: []genPersona{
				{Name: "小刘", Type: "person", Age: 29, Attrs: map[string]string{"job": "程序员", "city": "北京"}},
				{Name: "小陈", Type: "person", Age: 27, Attrs: map[string]string{"job": "产品经理"}},
			},
			Topics: []string{"工作", "跳槽", "面试", "项目"}, Places: []string{"北京", "上海", "深圳"},
			Purpose: "info_correction",
			Expect:  MetaExpect{MinEntities: 2, MinSegments: 2},
		},

		// 2 x 1000 messages
		{
			Name: "m08_family_tree", Desc: "Dense family relations over 50 sessions", Persona: "cat_girl",
			Total: 1000, ConvSize: 20,
			Personas: []genPersona{
				{Name: "爷爷", Type: "person", Age: 75},
				{Name: "奶奶", Type: "person", Age: 73},
				{Name: "爸爸", Type: "person", Age: 45, Attrs: map[string]string{"job": "教师"}},
				{Name: "妈妈", Type: "person", Age: 43, Attrs: map[string]string{"job": "护士"}},
				{Name: "哥哥", Type: "person", Age: 20, Attrs: map[string]string{"school": "清华大学"}},
				{Name: "妹妹", Type: "person", Age: 15, Attrs: map[string]string{"hobby": "钢琴"}},
			},
			Topics:  []string{"家庭", "过年", "生日", "旅行", "学校", "健康", "做饭"},
			Places:  []string{"老家", "北京", "学校", "医院"},
			Purpose: "relation_accumulation",
			Expect: MetaExpect{
				MinEntities: 5, MinRelations: 4, MinSegments: 8,
				Recall: []RecallExpect{
					{Text: "家庭", MinResults: 2},
					{Text: "学校", Labels: []string{"person:哥哥"}, MinResults: 1},
				},
			},
		},
		{
			Name: "m09_many_topics", Desc: "Many interleaved topics over 50 sessions", Persona: "assistant",
			Total: 1000, ConvSize: 20,
			Personas: []genPersona{
				{Name: "小何", Type: "person", Age: 31, Attrs: map[string]string{"job": "自由职业"}},
				{Name: "老张", Type: "person", Age: 55, Attrs: map[string]string{"role": "mentor"}},
				{Name: "小美", Type: "person", Age: 28, Attrs: map[string]string{"hobby": "跑步"}},
			},
			Topics:  []string{"天气", "新闻", "工作", "健康", "爱好", "理财", "学习", "旅行", "美食", "电影"},
			Places:  []string{"杭州", "苏州", "南京", "西湖"},
			Purpose: "topic_interleaving",
			Expect: MetaExpect{
				MinEntities: 3, MinSegments: 8,
				Recall: []RecallExpect{{Text: "工作", MinResults: 1}},
			},
		},

		// 1 x 10000 messages
		{
			Name: "m10_comprehensive", Desc: "Comprehensive stress test: 8 personas, 500 sessions, all patterns", Persona: "cat_girl",
			Total: 10000, ConvSize: 20,
			Personas: []genPersona{
				{Name: "小明", Type: "person", Age: 8, Attrs: map[string]string{"hobby": "恐龙", "dream": "古生物学家"}},
				{Name: "小红", Type: "person", Age: 6, Attrs: map[string]string{"hobby": "画画"}},
				{Name: "妈妈", Type: "person", Age: 38, Attrs: map[string]string{"skill": "做饭", "job": "设计师"}},
				{Name: "爸爸", Type: "person", Age: 40, Attrs: map[string]string{"hobby": "音乐", "job": "工程师"}},
				{Name: "爷爷", Type: "person", Age: 70},
				{Name: "奶奶", Type: "person", Age: 68},
				{Name: "老师", Type: "person", Age: 35, Attrs: map[string]string{"subject": "数学"}},
				{Name: "小胖", Type: "person", Age: 8, Attrs: map[string]string{"hobby": "足球"}},
			},
			Topics:  []string{"恐龙", "画画", "做饭", "音乐", "乐高", "学校", "考试", "运动", "旅行", "生日", "过年", "宠物"},
			Places:  []string{"学校", "博物馆", "公园", "奶奶家", "游乐场", "图书馆"},
			Purpose: "comprehensive",
			Expect: MetaExpect{
				MinEntities: 6, MinRelations: 5, MinSegments: 50,
				Recall: []RecallExpect{
					{Text: "恐龙", Labels: []string{"person:小明"}, MinResults: 2},
					{Text: "画画", Labels: []string{"person:小红"}, MinResults: 1},
					{Text: "做饭", MinResults: 1},
				},
			},
		},
	}
}

// ---------------------------------------------------------------------------
// Generate command
// ---------------------------------------------------------------------------

func generateMemoryCases(outDir string) error {
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return err
	}

	if err := generateSerialization(filepath.Join(outDir, "serialization")); err != nil {
		return fmt.Errorf("generate serialization: %w", err)
	}

	scenarios := buildScenarios()
	for _, sc := range scenarios {
		if err := generateScenario(outDir, sc); err != nil {
			return fmt.Errorf("generate %s: %w", sc.Name, err)
		}
	}

	// Pack into tar.gz alongside the output directory.
	tarPath := outDir + ".tar.gz"
	if err := packTarGz(outDir, tarPath); err != nil {
		return fmt.Errorf("pack tar.gz: %w", err)
	}

	fmt.Printf("\nGenerated %d scenarios, packed to %s\n", len(scenarios), tarPath)
	return nil
}

func generateScenario(outDir string, sc scenario) error {
	dir := filepath.Join(outDir, sc.Name)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	rng := rand.New(rand.NewSource(int64(hashStr(sc.Name))))
	gen := &memConvGen{
		rng:      rng,
		personas: sc.Personas,
		topics:   sc.Topics,
		places:   sc.Places,
		purpose:  sc.Purpose,
	}

	// Split total messages into conversations of ConvSize each.
	remaining := sc.Total
	convIdx := 0
	for remaining > 0 {
		convIdx++
		n := sc.ConvSize
		if n > remaining {
			n = remaining
		}

		// Pick a speaker focus for this conversation.
		// Rotate through personas round-robin (0-based) to ensure coverage.
		speaker := sc.Personas[(convIdx-1)%len(sc.Personas)]
		labels := []string{fmt.Sprintf("person:%s", speaker.Name)}

		msgs := gen.generateConv(n, speaker)
		cf := ConvFile{
			ConvID:   fmt.Sprintf("session-%03d", convIdx),
			Labels:   labels,
			Messages: msgs,
		}

		data, err := yaml.Marshal(cf)
		if err != nil {
			return fmt.Errorf("marshal conv %d: %w", convIdx, err)
		}
		path := filepath.Join(dir, fmt.Sprintf("conv_%03d.yaml", convIdx))
		if err := os.WriteFile(path, data, 0644); err != nil {
			return err
		}

		remaining -= len(msgs)
	}

	// Write meta.yaml.
	meta := Meta{
		Name:    sc.Name,
		Desc:    sc.Desc,
		Persona: sc.Persona,
		Total:   sc.Total,
		Convs:   convIdx,
		Expect:  sc.Expect,
	}
	data, err := yaml.Marshal(meta)
	if err != nil {
		return fmt.Errorf("marshal meta: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "meta.yaml"), data, 0644); err != nil {
		return err
	}

	fmt.Printf("  %s: %d messages in %d conversations\n", sc.Name, sc.Total, convIdx)
	return nil
}

// ---------------------------------------------------------------------------
// Conversation generator (outputs typed ConvMsg)
// ---------------------------------------------------------------------------

type memConvGen struct {
	rng      *rand.Rand
	personas []genPersona
	topics   []string
	places   []string
	purpose  string
	turn     int
}

func (g *memConvGen) generateConv(n int, focus genPersona) []ConvMsg {
	var msgs []ConvMsg

	// Opening greeting.
	msgs = append(msgs, g.greeting(focus)...)

	// For relation-focused conversations with multiple personas, ensure
	// a relation reveal happens early so entities are captured.
	if len(g.personas) > 1 && g.turn == 0 {
		msgs = append(msgs, g.relationReveal(focus)...)
	}

	for len(msgs) < n {
		if n-len(msgs) <= 1 {
			break
		}
		msgs = append(msgs, g.exchange(focus)...)
	}

	if len(msgs) > n {
		msgs = msgs[:n]
	}
	return msgs
}

func (g *memConvGen) greeting(focus genPersona) []ConvMsg {
	topic := g.pickTopic()
	greetings := []struct{ user, model string }{
		{"今天想聊聊%s的事", "好的，%s怎么了？"},
		{"跟你说个关于%s的事", "什么事呀？"},
		{"最近%s有什么新鲜事吗", "你说说看！"},
		{"想跟你聊聊%s", "好呀，说吧！"},
	}
	t := greetings[g.rng.Intn(len(greetings))]
	userContent := fmt.Sprintf(t.user, topic)
	modelContent := t.model
	if strings.Contains(modelContent, "%s") {
		modelContent = fmt.Sprintf(modelContent, topic)
	}
	return []ConvMsg{
		{Role: "user", Name: focus.Name, Content: userContent},
		{Role: "model", Content: modelContent},
	}
}

func (g *memConvGen) exchange(focus genPersona) []ConvMsg {
	g.turn++

	type patternFn func(genPersona) []ConvMsg
	patterns := []patternFn{
		g.attrReveal,
		g.topicChat,
		g.askAnswer,
		g.storyTelling,
	}
	if len(g.personas) > 1 {
		patterns = append(patterns, g.relationReveal)
	}
	if len(g.places) > 0 {
		patterns = append(patterns, g.placeChat)
	}
	if g.purpose == "info_correction" || g.purpose == "comprehensive" {
		patterns = append(patterns, g.correction)
	}

	fn := patterns[g.rng.Intn(len(patterns))]
	return fn(focus)
}

func (g *memConvGen) attrReveal(focus genPersona) []ConvMsg {
	p := g.pickOrFocus(focus)

	// Reveal from known attrs if available.
	if len(p.Attrs) > 0 {
		// Pick a random attr.
		keys := make([]string, 0, len(p.Attrs))
		for k := range p.Attrs {
			keys = append(keys, k)
		}
		k := keys[g.rng.Intn(len(keys))]
		v := p.Attrs[k]

		templates := []struct{ u, m string }{
			{"%s喜欢%s", "是吗，%s确实挺喜欢%s的"},
			{"%s最近在忙%s", "%s忙%s一定很充实"},
			{"%s说他特别喜欢%s", "原来%s喜欢%s呀"},
			{"%s跟我说过，他最大的爱好就是%s", "%s爱好%s挺好的"},
		}
		t := templates[g.rng.Intn(len(templates))]
		return []ConvMsg{
			{Role: "user", Name: focus.Name, Content: fmt.Sprintf(t.u, p.Name, v)},
			{Role: "model", Content: fmt.Sprintf(t.m, p.Name, v)},
		}
	}

	// Age reveal.
	if p.Age > 0 {
		templates := []struct{ u, m string }{
			{"%s今年%d岁了", "原来%s%d岁"},
			{"%s已经%d岁了", "%s%d岁了呀"},
		}
		t := templates[g.rng.Intn(len(templates))]
		return []ConvMsg{
			{Role: "user", Name: focus.Name, Content: fmt.Sprintf(t.u, p.Name, p.Age)},
			{Role: "model", Content: fmt.Sprintf(t.m, p.Name, p.Age)},
		}
	}

	return g.topicChat(focus)
}

func (g *memConvGen) topicChat(focus genPersona) []ConvMsg {
	topic := g.pickTopic()
	templates := []struct{ u, m string }{
		{"说到%s，我觉得挺有意思的", "是啊，%s确实值得聊聊"},
		{"最近%s有什么新动态吗", "我听说%s方面有不少变化"},
		{"你了解%s吗", "了解一些，%s挺热门的"},
		{"我对%s很感兴趣", "%s是个好话题"},
		{"%s太好玩了！", "是呀，%s确实很有趣！"},
	}
	t := templates[g.rng.Intn(len(templates))]
	return []ConvMsg{
		{Role: "user", Name: focus.Name, Content: fmt.Sprintf(t.u, topic)},
		{Role: "model", Content: fmt.Sprintf(t.m, topic)},
	}
}

func (g *memConvGen) askAnswer(focus genPersona) []ConvMsg {
	p := g.pickOrFocus(focus)
	questions := []struct{ q, a string }{
		{"%s平时有什么爱好？", "%s的爱好挺多的"},
		{"%s最近怎么样？", "%s最近挺好的"},
		{"%s有没有什么计划？", "%s应该有一些计划"},
		{"%s喜欢吃什么？", "%s口味挺好的"},
		{"%s周末一般做什么？", "%s周末过得挺充实的"},
	}
	q := questions[g.rng.Intn(len(questions))]
	return []ConvMsg{
		{Role: "user", Name: focus.Name, Content: fmt.Sprintf(q.q, p.Name)},
		{Role: "model", Content: fmt.Sprintf(q.a, p.Name)},
	}
}

func (g *memConvGen) storyTelling(focus genPersona) []ConvMsg {
	p := g.pickOrFocus(focus)
	topic := g.pickTopic()
	stories := []struct{ s1, s2, s3, s4 string }{
		{"上次%s跟我说了一件关于%s的事", "什么事？", "%s说%s让他学到了很多", "这样啊"},
		{"有一次%s因为%s闹了个笑话", "哈哈什么笑话", "就是%s在%s的时候出了个糗", "听起来很有趣"},
		{"我记得%s第一次接触%s的时候", "然后呢？", "%s当时觉得%s特别新奇", "每个人都有第一次嘛"},
		{"%s今天跟我说了%s的事", "说了什么？", "%s说%s最近变化挺大的", "是嘛，那挺好的"},
	}
	s := stories[g.rng.Intn(len(stories))]
	return []ConvMsg{
		{Role: "user", Name: focus.Name, Content: fmt.Sprintf(s.s1, p.Name, topic)},
		{Role: "model", Content: s.s2},
		{Role: "user", Name: focus.Name, Content: fmt.Sprintf(s.s3, p.Name, topic)},
		{Role: "model", Content: s.s4},
	}
}

func (g *memConvGen) relationReveal(focus genPersona) []ConvMsg {
	if len(g.personas) < 2 {
		return g.topicChat(focus)
	}
	p1 := g.pickPersona()
	p2 := g.pickPersona()
	for p2.Name == p1.Name && len(g.personas) > 1 {
		p2 = g.pickPersona()
	}
	relTypes := []string{"朋友", "同学", "同事", "邻居", "兄妹", "姐弟", "亲戚"}
	rel := relTypes[g.rng.Intn(len(relTypes))]
	return []ConvMsg{
		{Role: "user", Name: focus.Name, Content: fmt.Sprintf("%s和%s是%s", p1.Name, p2.Name, rel)},
		{Role: "model", Content: fmt.Sprintf("原来%s和%s是%s关系", p1.Name, p2.Name, rel)},
	}
}

func (g *memConvGen) placeChat(focus genPersona) []ConvMsg {
	place := g.places[g.rng.Intn(len(g.places))]
	p := g.pickOrFocus(focus)
	templates := []struct{ u, m string }{
		{"%s去过%s", "%s挺美的"},
		{"%s打算去%s玩", "%s是个好地方"},
		{"%s在%s住过一段时间", "在%s生活应该很有意思"},
		{"上周%s去了%s", "%s不错呢"},
	}
	t := templates[g.rng.Intn(len(templates))]
	return []ConvMsg{
		{Role: "user", Name: focus.Name, Content: fmt.Sprintf(t.u, p.Name, place)},
		{Role: "model", Content: fmt.Sprintf(t.m, place)},
	}
}

func (g *memConvGen) correction(focus genPersona) []ConvMsg {
	p := g.pickOrFocus(focus)
	corrections := []struct{ s1, s2, s3, s4 string }{
		{"%s是在北京工作的", "好的", "哦不对，%s是在上海工作的，我记错了", "没关系"},
		{"%s今年25岁", "好的", "等等，%s应该是26岁，上个月刚过生日", "生日快乐"},
		{"%s学的是数学", "数学不错", "不对，%s学的是物理，我搞混了", "物理也很好"},
		{"%s在一家大公司", "挺好的", "其实%s自己创业了，上周才跟我说的", "创业挺勇敢的"},
	}
	c := corrections[g.rng.Intn(len(corrections))]
	return []ConvMsg{
		{Role: "user", Name: focus.Name, Content: fmt.Sprintf(c.s1, p.Name)},
		{Role: "model", Content: c.s2},
		{Role: "user", Name: focus.Name, Content: fmt.Sprintf(c.s3, p.Name)},
		{Role: "model", Content: c.s4},
	}
}

func (g *memConvGen) pickPersona() genPersona {
	return g.personas[g.rng.Intn(len(g.personas))]
}

func (g *memConvGen) pickOrFocus(focus genPersona) genPersona {
	// 70% chance to talk about the focus persona, 30% someone else.
	if g.rng.Float32() < 0.7 || len(g.personas) <= 1 {
		return focus
	}
	return g.pickPersona()
}

func (g *memConvGen) pickTopic() string {
	if len(g.topics) == 0 {
		return "事情"
	}
	return g.topics[g.rng.Intn(len(g.topics))]
}

// ---------------------------------------------------------------------------
// Tar packing
// ---------------------------------------------------------------------------

func packTarGz(srcDir, tarPath string) error {
	f, err := os.Create(tarPath)
	if err != nil {
		return err
	}
	defer f.Close()

	gw := gzip.NewWriter(f)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	base := filepath.Base(srcDir)

	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Relative path inside the tar starts with the base directory name.
		rel, err := filepath.Rel(filepath.Dir(srcDir), path)
		if err != nil {
			return err
		}
		_ = base

		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = rel

		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		_, err = tw.Write(data)
		return err
	})
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func hashStr(s string) uint32 {
	var h uint32 = 2166136261
	for i := range len(s) {
		h ^= uint32(s[i])
		h *= 16777619
	}
	return h
}

// ---------------------------------------------------------------------------
// Serialization Generation
// ---------------------------------------------------------------------------

func generateSerialization(outDir string) error {
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return err
	}

	msgs := map[string]memory.Message{
		"message_user": {
			Role:      "user",
			Name:      "",
			Content:   "hello",
			Timestamp: 100,
		},
		"message_model": {
			Role:      "model",
			Name:      "",
			Content:   "response",
			Timestamp: 200,
		},
		"message_tool": {
			Role:         "tool",
			Name:         "",
			Content:      "result",
			Timestamp:    300,
			ToolCallID:   "tc1",
			ToolCallName: "fn1",
			ToolCallArgs: "{}",
			ToolResultID: "tr1",
		},
	}

	for name, msg := range msgs {
		data, err := msgpack.Marshal(&msg)
		if err != nil {
			return err
		}
		path := filepath.Join(outDir, name+".msgpack")
		if err := os.WriteFile(path, data, 0644); err != nil {
			return err
		}
	}

	// Generate shared KV key golden file used by TX.5 byte-exact checks.
	keysDir := filepath.Join(filepath.Dir(outDir), "keys")
	if err := os.MkdirAll(keysDir, 0755); err != nil {
		return err
	}

	keysFile := filepath.Join(keysDir, "conv_msg_keys.txt")
	var keyLines []string
	keyLines = append(keyLines, fmt.Sprintf("mem:p1:conv:c1:msg:%020d", 123456789))
	keyLines = append(keyLines, fmt.Sprintf("mem:p1:conv:c1:msg:%020d", 987654321))

	if err := os.WriteFile(keysFile, []byte(strings.Join(keyLines, "\n")), 0644); err != nil {
		return err
	}

	return nil
}
