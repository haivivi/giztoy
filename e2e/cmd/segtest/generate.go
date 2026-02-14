package main

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"

	"github.com/goccy/go-yaml"
)

// generateLongCases generates 30 long test case YAML files into dir.
//
// Structure:
//   - 10 cases x ~100 turns
//   - 10 cases x ~500 turns
//   - 10 cases x ~1000 turns
func generateLongCases(dir string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	specs := buildLongSpecs()
	for _, spec := range specs {
		tc := generateCase(spec)
		data, err := yaml.Marshal(tc)
		if err != nil {
			return fmt.Errorf("marshal %s: %w", spec.Name, err)
		}
		path := filepath.Join(dir, spec.Name+".yaml")
		if err := os.WriteFile(path, data, 0644); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
		fmt.Printf("  generated %s (%d turns)\n", spec.Name, len(tc.Messages))
	}

	fmt.Printf("\nGenerated %d long test cases in %s\n", len(specs), dir)
	return nil
}

// ---------------------------------------------------------------------------
// Spec: describes what to generate
// ---------------------------------------------------------------------------

type longSpec struct {
	Name        string
	Desc        string
	Turns       int
	Personas    []persona
	Topics      []string
	Places      []string
	Purpose     string // e.g., "accumulation", "topic_drift", "entity_evolution"
	ExpectSetup expectSetup
}

type persona struct {
	Name   string
	Type   string // "person", "animal", etc.
	Attrs  map[string]any
	Rels   []relSpec
}

type relSpec struct {
	To      string
	RelType string
}

type expectSetup struct {
	MinEntities     int
	EntitiesContain []string
	SummaryContains []string
	LabelsContain   []string
	Relations       []ExpectedRelation
}

func buildLongSpecs() []longSpec {
	var specs []longSpec

	// ----- 100-turn cases (l01 - l10) -----
	specs = append(specs, longSpec{
		Name: "l01_accumulate_100", Desc: "Accumulate facts about one person over 100 turns", Turns: 100,
		Purpose: "accumulation",
		Personas: []persona{{Name: "小明", Type: "person", Attrs: map[string]any{"age": 10, "hobby": "画画", "school": "北京小学"}}},
		Topics: []string{"画画", "学校生活"}, Places: []string{"北京"},
		ExpectSetup: expectSetup{MinEntities: 1, EntitiesContain: []string{"person:小明"}, SummaryContains: []string{"小明"}, LabelsContain: []string{"person:小明"}},
	})
	specs = append(specs, longSpec{
		Name: "l02_two_people_100", Desc: "Two people discussed alternately over 100 turns", Turns: 100,
		Purpose: "multi_entity",
		Personas: []persona{
			{Name: "Alice", Type: "person", Attrs: map[string]any{"age": 30, "job": "engineer"}},
			{Name: "Bob", Type: "person", Attrs: map[string]any{"age": 28, "job": "designer"}, Rels: []relSpec{{To: "person:Alice", RelType: "colleague"}}},
		},
		Topics: []string{"work", "project"}, Places: []string{"San Francisco"},
		ExpectSetup: expectSetup{MinEntities: 2, EntitiesContain: []string{"person:Alice", "person:Bob"}, SummaryContains: []string{"Alice", "Bob"}},
	})
	specs = append(specs, longSpec{
		Name: "l03_topic_drift_100", Desc: "Topics gradually shift over 100 turns", Turns: 100,
		Purpose: "topic_drift",
		Personas: []persona{{Name: "小红", Type: "person", Attrs: map[string]any{"age": 25}}},
		Topics: []string{"旅行", "美食", "电影", "运动"}, Places: []string{"上海", "成都", "广州"},
		ExpectSetup: expectSetup{MinEntities: 1, EntitiesContain: []string{"person:小红"}, SummaryContains: []string{"小红"}},
	})
	specs = append(specs, longSpec{
		Name: "l04_family_100", Desc: "Family information revealed gradually over 100 turns", Turns: 100,
		Purpose: "relation_accumulation",
		Personas: []persona{
			{Name: "张伟", Type: "person", Attrs: map[string]any{"age": 40, "job": "教师"}},
			{Name: "李芳", Type: "person", Attrs: map[string]any{"age": 38, "job": "护士"}, Rels: []relSpec{{To: "person:张伟", RelType: "spouse"}}},
			{Name: "张小宝", Type: "person", Attrs: map[string]any{"age": 8}, Rels: []relSpec{{To: "person:张伟", RelType: "child"}}},
		},
		ExpectSetup: expectSetup{MinEntities: 2, EntitiesContain: []string{"person:张伟", "person:李芳"}, SummaryContains: []string{"张伟"}},
	})
	specs = append(specs, longSpec{
		Name: "l05_pet_stories_100", Desc: "Stories about pets over 100 turns", Turns: 100,
		Purpose: "non_person_entities",
		Personas: []persona{
			{Name: "小李", Type: "person", Attrs: map[string]any{"age": 35}},
			{Name: "旺财", Type: "animal", Attrs: map[string]any{"species": "金毛犬", "age": 3}},
		},
		Topics: []string{"宠物", "遛狗", "动物医院"},
		ExpectSetup: expectSetup{MinEntities: 1, EntitiesContain: []string{"person:小李"}, SummaryContains: []string{"小李"}},
	})
	specs = append(specs, longSpec{
		Name: "l06_school_life_100", Desc: "School life discussions over 100 turns", Turns: 100,
		Purpose: "context_accumulation",
		Personas: []persona{{Name: "小林", Type: "person", Attrs: map[string]any{"age": 16, "grade": "高一"}}},
		Topics: []string{"数学", "物理", "英语", "考试"}, Places: []string{"学校", "图书馆"},
		ExpectSetup: expectSetup{MinEntities: 1, EntitiesContain: []string{"person:小林"}, SummaryContains: []string{"小林"}},
	})
	specs = append(specs, longSpec{
		Name: "l07_work_life_100", Desc: "Work and life balance over 100 turns", Turns: 100,
		Purpose: "multi_topic",
		Personas: []persona{{Name: "Emma", Type: "person", Attrs: map[string]any{"age": 32, "job": "product manager"}}},
		Topics: []string{"meetings", "deadlines", "yoga", "cooking"}, Places: []string{"office", "home", "gym"},
		ExpectSetup: expectSetup{MinEntities: 1, EntitiesContain: []string{"person:Emma"}, SummaryContains: []string{"Emma"}},
	})
	specs = append(specs, longSpec{
		Name: "l08_travel_plan_100", Desc: "Planning a trip over 100 turns", Turns: 100,
		Purpose: "place_extraction",
		Personas: []persona{{Name: "小王", Type: "person", Attrs: map[string]any{"age": 27}}},
		Topics: []string{"旅行计划"}, Places: []string{"东京", "京都", "大阪", "北海道"},
		ExpectSetup: expectSetup{MinEntities: 1, EntitiesContain: []string{"person:小王"}, SummaryContains: []string{"小王"}},
	})
	specs = append(specs, longSpec{
		Name: "l09_hobby_deep_100", Desc: "Deep dive into hobbies over 100 turns", Turns: 100,
		Purpose: "attribute_depth",
		Personas: []persona{{Name: "小陈", Type: "person", Attrs: map[string]any{"age": 22}}},
		Topics: []string{"摄影", "器材", "后期", "展览"},
		ExpectSetup: expectSetup{MinEntities: 1, EntitiesContain: []string{"person:小陈"}, SummaryContains: []string{"小陈"}},
	})
	specs = append(specs, longSpec{
		Name: "l10_correction_100", Desc: "Corrections and updates over 100 turns", Turns: 100,
		Purpose: "info_correction",
		Personas: []persona{{Name: "小刘", Type: "person", Attrs: map[string]any{"age": 29, "job": "程序员"}}},
		Topics: []string{"工作", "跳槽", "面试"},
		ExpectSetup: expectSetup{MinEntities: 1, EntitiesContain: []string{"person:小刘"}, SummaryContains: []string{"小刘"}},
	})

	// ----- 500-turn cases (l11 - l20) -----
	specs = append(specs, longSpec{
		Name: "l11_multi_persona_500", Desc: "5 personas discussed over 500 turns", Turns: 500,
		Purpose: "multi_persona",
		Personas: []persona{
			{Name: "张三", Type: "person", Attrs: map[string]any{"age": 30}},
			{Name: "李四", Type: "person", Attrs: map[string]any{"age": 28}, Rels: []relSpec{{To: "person:张三", RelType: "friend"}}},
			{Name: "王五", Type: "person", Attrs: map[string]any{"age": 35}},
			{Name: "赵六", Type: "person", Attrs: map[string]any{"age": 26}},
			{Name: "钱七", Type: "person", Attrs: map[string]any{"age": 32}},
		},
		Topics: []string{"聚会", "旅行", "工作", "美食"},
		ExpectSetup: expectSetup{MinEntities: 3, EntitiesContain: []string{"person:张三", "person:李四", "person:王五"}, SummaryContains: []string{"张三"}},
	})
	specs = append(specs, longSpec{
		Name: "l12_entity_evolution_500", Desc: "Entity attributes evolve over 500 turns", Turns: 500,
		Purpose: "entity_evolution",
		Personas: []persona{{Name: "小杰", Type: "person", Attrs: map[string]any{"age": 18, "school": "清华大学"}}},
		Topics: []string{"大学生活", "实习", "论文", "毕业"},
		ExpectSetup: expectSetup{MinEntities: 1, EntitiesContain: []string{"person:小杰"}, SummaryContains: []string{"小杰"}},
	})
	specs = append(specs, longSpec{
		Name: "l13_repeated_mentions_500", Desc: "Same entities mentioned repeatedly with new info", Turns: 500,
		Purpose: "repeated_mentions",
		Personas: []persona{
			{Name: "David", Type: "person", Attrs: map[string]any{"age": 40, "job": "CEO"}},
			{Name: "Sarah", Type: "person", Attrs: map[string]any{"age": 37, "job": "CTO"}, Rels: []relSpec{{To: "person:David", RelType: "colleague"}}},
		},
		Topics: []string{"startup", "funding", "product launch"},
		ExpectSetup: expectSetup{MinEntities: 2, EntitiesContain: []string{"person:David", "person:Sarah"}, SummaryContains: []string{"David"}},
	})
	specs = append(specs, longSpec{
		Name: "l14_mixed_lang_500", Desc: "Mixed Chinese/English conversation over 500 turns", Turns: 500,
		Purpose: "code_switching",
		Personas: []persona{{Name: "小周", Type: "person", Attrs: map[string]any{"age": 24, "job": "developer"}}},
		Topics: []string{"coding", "debugging", "开源项目", "技术分享"},
		ExpectSetup: expectSetup{MinEntities: 1, EntitiesContain: []string{"person:小周"}, SummaryContains: []string{"小周"}},
	})
	specs = append(specs, longSpec{
		Name: "l15_many_relations_500", Desc: "Dense relation network over 500 turns", Turns: 500,
		Purpose: "relation_density",
		Personas: []persona{
			{Name: "老王", Type: "person", Attrs: map[string]any{"age": 55, "role": "班主任"}},
			{Name: "小孙", Type: "person", Attrs: map[string]any{"age": 12}, Rels: []relSpec{{To: "person:老王", RelType: "student"}}},
			{Name: "小周", Type: "person", Attrs: map[string]any{"age": 12}, Rels: []relSpec{{To: "person:小孙", RelType: "classmate"}}},
			{Name: "小吴", Type: "person", Attrs: map[string]any{"age": 11}, Rels: []relSpec{{To: "person:小孙", RelType: "classmate"}}},
		},
		ExpectSetup: expectSetup{MinEntities: 3, EntitiesContain: []string{"person:老王", "person:小孙"}, SummaryContains: []string{"老王"}},
	})
	specs = append(specs, longSpec{
		Name: "l16_daily_life_500", Desc: "Daily life routine discussed over 500 turns", Turns: 500,
		Purpose: "daily_routine",
		Personas: []persona{{Name: "小黄", Type: "person", Attrs: map[string]any{"age": 30}}},
		Topics: []string{"早起", "通勤", "午餐", "健身", "做饭"}, Places: []string{"家", "公司", "健身房"},
		ExpectSetup: expectSetup{MinEntities: 1, EntitiesContain: []string{"person:小黄"}, SummaryContains: []string{"小黄"}},
	})
	specs = append(specs, longSpec{
		Name: "l17_health_tracking_500", Desc: "Health and medical discussions over 500 turns", Turns: 500,
		Purpose: "numeric_tracking",
		Personas: []persona{{Name: "老李", Type: "person", Attrs: map[string]any{"age": 60}}},
		Topics: []string{"血压", "体重", "运动", "饮食", "体检"},
		ExpectSetup: expectSetup{MinEntities: 1, EntitiesContain: []string{"person:老李"}, SummaryContains: []string{"老李"}},
	})
	specs = append(specs, longSpec{
		Name: "l18_project_timeline_500", Desc: "Project development timeline over 500 turns", Turns: 500,
		Purpose: "temporal_tracking",
		Personas: []persona{
			{Name: "Mike", Type: "person", Attrs: map[string]any{"age": 35, "role": "tech lead"}},
			{Name: "Jenny", Type: "person", Attrs: map[string]any{"age": 29, "role": "designer"}, Rels: []relSpec{{To: "person:Mike", RelType: "colleague"}}},
		},
		Topics: []string{"sprint planning", "code review", "release"}, Places: []string{"office"},
		ExpectSetup: expectSetup{MinEntities: 2, EntitiesContain: []string{"person:Mike", "person:Jenny"}, SummaryContains: []string{"Mike"}},
	})
	specs = append(specs, longSpec{
		Name: "l19_hobby_network_500", Desc: "Hobby community and friendships over 500 turns", Turns: 500,
		Purpose: "social_network",
		Personas: []persona{
			{Name: "小赵", Type: "person", Attrs: map[string]any{"age": 28}},
			{Name: "小钱", Type: "person", Attrs: map[string]any{"age": 26}, Rels: []relSpec{{To: "person:小赵", RelType: "friend"}}},
		},
		Topics: []string{"跑步", "马拉松", "骑行", "登山"},
		ExpectSetup: expectSetup{MinEntities: 2, EntitiesContain: []string{"person:小赵", "person:小钱"}, SummaryContains: []string{"小赵"}},
	})
	specs = append(specs, longSpec{
		Name: "l20_learning_path_500", Desc: "Learning journey over 500 turns", Turns: 500,
		Purpose: "progression_tracking",
		Personas: []persona{{Name: "小吴", Type: "person", Attrs: map[string]any{"age": 20}}},
		Topics: []string{"编程入门", "数据结构", "算法", "项目实战", "面试"},
		ExpectSetup: expectSetup{MinEntities: 1, EntitiesContain: []string{"person:小吴"}, SummaryContains: []string{"小吴"}},
	})

	// ----- 1000-turn cases (l21 - l30) -----
	specs = append(specs, longSpec{
		Name: "l21_stress_entities_1000", Desc: "Stress test with many entities over 1000 turns", Turns: 1000,
		Purpose: "entity_count_scaling",
		Personas: buildManyPersonas(10),
		Topics: []string{"公司", "团建", "年会", "项目"},
		ExpectSetup: expectSetup{MinEntities: 5, SummaryContains: []string{"公司"}},
	})
	specs = append(specs, longSpec{
		Name: "l22_dense_relations_1000", Desc: "Dense relation network over 1000 turns", Turns: 1000,
		Purpose: "relation_density",
		Personas: buildFamilyTree(),
		ExpectSetup: expectSetup{MinEntities: 4},
	})
	specs = append(specs, longSpec{
		Name: "l23_info_overload_1000", Desc: "Information overload and retention over 1000 turns", Turns: 1000,
		Purpose: "information_retention",
		Personas: []persona{{Name: "小张", Type: "person", Attrs: map[string]any{"age": 25}}},
		Topics: []string{"历史", "科学", "艺术", "体育", "音乐", "电影", "美食", "旅行"},
		ExpectSetup: expectSetup{MinEntities: 1, EntitiesContain: []string{"person:小张"}, SummaryContains: []string{"小张"}},
	})
	specs = append(specs, longSpec{
		Name: "l24_contradiction_storm_1000", Desc: "Many corrections and contradictions over 1000 turns", Turns: 1000,
		Purpose: "contradiction_handling",
		Personas: []persona{{Name: "小郑", Type: "person", Attrs: map[string]any{"age": 33}}},
		Topics: []string{"工作变动", "搬家", "感情", "健康"},
		ExpectSetup: expectSetup{MinEntities: 1, EntitiesContain: []string{"person:小郑"}, SummaryContains: []string{"小郑"}},
	})
	specs = append(specs, longSpec{
		Name: "l25_bilingual_1000", Desc: "Extensive bilingual conversation over 1000 turns", Turns: 1000,
		Purpose: "bilingual_stress",
		Personas: []persona{
			{Name: "Tom", Type: "person", Attrs: map[string]any{"age": 28}},
			{Name: "小美", Type: "person", Attrs: map[string]any{"age": 26}, Rels: []relSpec{{To: "person:Tom", RelType: "friend"}}},
		},
		Topics: []string{"language exchange", "文化差异", "food", "旅行"},
		ExpectSetup: expectSetup{MinEntities: 2, EntitiesContain: []string{"person:Tom", "person:小美"}, SummaryContains: []string{"Tom"}},
	})
	specs = append(specs, longSpec{
		Name: "l26_multi_topic_1000", Desc: "Many interleaved topics over 1000 turns", Turns: 1000,
		Purpose: "topic_interleaving",
		Personas: []persona{{Name: "小何", Type: "person", Attrs: map[string]any{"age": 31}}},
		Topics: []string{"天气", "新闻", "工作", "家庭", "爱好", "理财", "健康", "学习", "旅行", "社交"},
		ExpectSetup: expectSetup{MinEntities: 1, EntitiesContain: []string{"person:小何"}, SummaryContains: []string{"小何"}},
	})
	specs = append(specs, longSpec{
		Name: "l27_temporal_1000", Desc: "Many temporal facts spread over 1000 turns", Turns: 1000,
		Purpose: "temporal_density",
		Personas: []persona{{Name: "小冯", Type: "person", Attrs: map[string]any{"age": 45}}},
		Topics: []string{"日程", "会议", "生日", "纪念日", "旅行计划"},
		ExpectSetup: expectSetup{MinEntities: 1, EntitiesContain: []string{"person:小冯"}, SummaryContains: []string{"小冯"}},
	})
	specs = append(specs, longSpec{
		Name: "l28_social_graph_1000", Desc: "Large social graph over 1000 turns", Turns: 1000,
		Purpose: "social_graph_scaling",
		Personas: buildManyPersonas(15),
		Topics: []string{"聚会", "婚礼", "同学会", "生日"},
		ExpectSetup: expectSetup{MinEntities: 5},
	})
	specs = append(specs, longSpec{
		Name: "l29_emotional_journey_1000", Desc: "Emotional journey with factual grounding over 1000 turns", Turns: 1000,
		Purpose: "emotion_fact_separation",
		Personas: []persona{
			{Name: "小曹", Type: "person", Attrs: map[string]any{"age": 27}},
			{Name: "小丁", Type: "person", Attrs: map[string]any{"age": 25}, Rels: []relSpec{{To: "person:小曹", RelType: "friend"}}},
		},
		Topics: []string{"考研", "失恋", "找工作", "新开始"},
		ExpectSetup: expectSetup{MinEntities: 2, EntitiesContain: []string{"person:小曹", "person:小丁"}, SummaryContains: []string{"小曹"}},
	})
	specs = append(specs, longSpec{
		Name: "l30_everything_1000", Desc: "Comprehensive stress test with all dimensions", Turns: 1000,
		Purpose: "comprehensive",
		Personas: func() []persona {
			pp := buildManyPersonas(8)
			pp[0].Rels = []relSpec{{To: fmt.Sprintf("person:%s", pp[1].Name), RelType: "friend"}}
			return pp
		}(),
		Topics: []string{"工作", "学习", "旅行", "健康", "家庭", "爱好", "理财"},
		Places: []string{"北京", "上海", "深圳"},
		ExpectSetup: expectSetup{MinEntities: 3},
	})

	return specs
}

func buildManyPersonas(n int) []persona {
	names := []string{"张三", "李四", "王五", "赵六", "钱七", "孙八", "周九",
		"吴十", "郑一", "冯二", "陈三", "褚四", "卫五", "蒋六", "沈七"}
	var pp []persona
	for i := range n {
		if i >= len(names) {
			break
		}
		pp = append(pp, persona{
			Name:  names[i],
			Type:  "person",
			Attrs: map[string]any{"age": 25 + i*3},
		})
	}
	return pp
}

func buildFamilyTree() []persona {
	return []persona{
		{Name: "爷爷", Type: "person", Attrs: map[string]any{"age": 75}},
		{Name: "奶奶", Type: "person", Attrs: map[string]any{"age": 73}, Rels: []relSpec{{To: "person:爷爷", RelType: "spouse"}}},
		{Name: "爸爸", Type: "person", Attrs: map[string]any{"age": 45}, Rels: []relSpec{{To: "person:爷爷", RelType: "child"}}},
		{Name: "妈妈", Type: "person", Attrs: map[string]any{"age": 43}, Rels: []relSpec{{To: "person:爸爸", RelType: "spouse"}}},
		{Name: "哥哥", Type: "person", Attrs: map[string]any{"age": 20}, Rels: []relSpec{{To: "person:爸爸", RelType: "child"}}},
		{Name: "妹妹", Type: "person", Attrs: map[string]any{"age": 15}, Rels: []relSpec{{To: "person:哥哥", RelType: "sibling"}}},
	}
}

// ---------------------------------------------------------------------------
// Conversation generation
// ---------------------------------------------------------------------------

func generateCase(spec longSpec) TestCase {
	rng := rand.New(rand.NewSource(int64(hashString(spec.Name))))
	msgs := generateMessages(rng, spec)

	tc := TestCase{
		Name:     spec.Name,
		Desc:     spec.Desc,
		Tier:     "long",
		Messages: msgs,
		Expect: Expect{
			MinEntities:     spec.ExpectSetup.MinEntities,
			EntitiesContain: spec.ExpectSetup.EntitiesContain,
			SummaryContains: spec.ExpectSetup.SummaryContains,
			LabelsContain:   spec.ExpectSetup.LabelsContain,
			RelationsContain: spec.ExpectSetup.Relations,
		},
	}
	return tc
}

func generateMessages(rng *rand.Rand, spec longSpec) []string {
	var msgs []string
	gen := &convGen{
		rng:      rng,
		personas: spec.Personas,
		topics:   spec.Topics,
		places:   spec.Places,
		purpose:  spec.Purpose,
	}

	// Opening.
	msgs = append(msgs, gen.greeting()...)

	// Body turns.
	for len(msgs) < spec.Turns {
		remaining := spec.Turns - len(msgs)
		if remaining <= 2 {
			break
		}
		msgs = append(msgs, gen.nextExchange()...)
	}

	// Trim to exact target.
	if len(msgs) > spec.Turns {
		msgs = msgs[:spec.Turns]
	}

	return msgs
}

type convGen struct {
	rng      *rand.Rand
	personas []persona
	topics   []string
	places   []string
	purpose  string
	turn     int
}

func (g *convGen) greeting() []string {
	p := g.pickPersona()
	return []string{
		fmt.Sprintf("user: 我想聊聊%s的事情", p.Name),
		fmt.Sprintf("assistant: 好的，关于%s你想聊什么？", p.Name),
	}
}

func (g *convGen) nextExchange() []string {
	g.turn++

	// Pick patterns based on purpose and randomness.
	patterns := []func() []string{
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
	if g.purpose == "info_correction" || g.purpose == "contradiction_handling" {
		patterns = append(patterns, g.correction)
	}

	fn := patterns[g.rng.Intn(len(patterns))]
	return fn()
}

func (g *convGen) attrReveal() []string {
	p := g.pickPersona()
	templates := []struct{ user, assistant string }{
		{"%s今年%v岁了", "原来%s%v岁"},
		{"%s喜欢%s", "是吗，%s还挺喜欢%s的"},
		{"%s在%s工作", "在%s工作挺好的"},
		{"%s最近在学%s", "%s学%s一定很有趣"},
		{"%s每天%s", "%s的生活挺规律的"},
	}

	t := templates[g.rng.Intn(len(templates))]
	topic := g.pickTopic()

	var user, assistant string
	if strings.Contains(t.user, "%v") {
		// Numeric attr
		age := 20 + g.rng.Intn(40)
		if v, ok := p.Attrs["age"]; ok {
			age, _ = toInt(v)
		}
		user = fmt.Sprintf("user: "+t.user, p.Name, age)
		assistant = fmt.Sprintf("assistant: "+t.assistant, p.Name, age)
	} else {
		user = fmt.Sprintf("user: "+t.user, p.Name, topic)
		assistant = fmt.Sprintf("assistant: "+t.assistant, p.Name, topic)
	}

	return []string{user, assistant}
}

func (g *convGen) topicChat() []string {
	topic := g.pickTopic()
	templates := []struct{ user, assistant string }{
		{"说到%s，我觉得挺有意思的", "是啊，%s确实值得聊聊"},
		{"最近%s有什么新动态吗", "我听说%s方面有不少变化"},
		{"你了解%s吗", "了解一些，%s挺热门的"},
		{"我对%s很感兴趣", "%s是个好话题"},
	}
	t := templates[g.rng.Intn(len(templates))]
	return []string{
		fmt.Sprintf("user: "+t.user, topic),
		fmt.Sprintf("assistant: "+t.assistant, topic),
	}
}

func (g *convGen) askAnswer() []string {
	p := g.pickPersona()
	questions := []struct{ q, a string }{
		{"%s平时有什么爱好？", "%s的爱好挺多的"},
		{"%s住在哪里？", "%s住的地方还不错"},
		{"%s最近怎么样？", "%s最近挺好的"},
		{"%s有没有什么计划？", "%s应该有一些计划"},
		{"%s工作忙吗？", "%s工作确实挺忙的"},
	}
	q := questions[g.rng.Intn(len(questions))]
	return []string{
		fmt.Sprintf("user: "+q.q, p.Name),
		fmt.Sprintf("assistant: "+q.a, p.Name),
	}
}

func (g *convGen) storyTelling() []string {
	p := g.pickPersona()
	topic := g.pickTopic()
	stories := []struct{ s1, s2, s3, s4 string }{
		{"上次%s跟我说了一件事关于%s的", "什么事？", "%s说%s让他学到了很多", "这样啊"},
		{"有一次%s因为%s闹了个笑话", "哈哈什么笑话", "就是%s在%s的时候出了个糗", "听起来很有趣"},
		{"我记得%s第一次接触%s的时候", "然后呢？", "%s当时对%s完全不了解", "每个人都有第一次嘛"},
	}
	s := stories[g.rng.Intn(len(stories))]
	return []string{
		fmt.Sprintf("user: "+s.s1, p.Name, topic),
		fmt.Sprintf("assistant: "+s.s2),
		fmt.Sprintf("user: "+s.s3, p.Name, topic),
		fmt.Sprintf("assistant: "+s.s4),
	}
}

func (g *convGen) relationReveal() []string {
	if len(g.personas) < 2 {
		return g.topicChat()
	}
	p1 := g.personas[g.rng.Intn(len(g.personas))]
	p2 := g.personas[g.rng.Intn(len(g.personas))]
	for p2.Name == p1.Name && len(g.personas) > 1 {
		p2 = g.personas[g.rng.Intn(len(g.personas))]
	}

	relTypes := []string{"朋友", "同学", "同事", "邻居", "认识"}
	rel := relTypes[g.rng.Intn(len(relTypes))]

	return []string{
		fmt.Sprintf("user: %s和%s是%s", p1.Name, p2.Name, rel),
		fmt.Sprintf("assistant: 原来%s和%s是%s关系", p1.Name, p2.Name, rel),
	}
}

func (g *convGen) placeChat() []string {
	place := g.places[g.rng.Intn(len(g.places))]
	p := g.pickPersona()
	templates := []struct{ user, assistant string }{
		{"%s去过%s", "%s挺美的"},
		{"%s打算去%s玩", "%s是个好地方"},
		{"%s在%s住过一段时间", "在%s生活应该很有意思"},
	}
	t := templates[g.rng.Intn(len(templates))]
	return []string{
		fmt.Sprintf("user: "+t.user, p.Name, place),
		fmt.Sprintf("assistant: "+t.assistant, place),
	}
}

func (g *convGen) correction() []string {
	p := g.pickPersona()
	corrections := []struct{ s1, s2, s3, s4 string }{
		{"%s是在北京工作的", "好的", "哦不对，%s是在上海工作的，我记错了", "没关系"},
		{"%s今年25岁", "好的", "等等，%s应该是26岁，上个月刚过生日", "生日快乐"},
		{"%s学的是数学", "数学不错", "不对，%s学的是物理，我搞混了", "物理也很好"},
	}
	c := corrections[g.rng.Intn(len(corrections))]
	return []string{
		fmt.Sprintf("user: "+c.s1, p.Name),
		fmt.Sprintf("assistant: "+c.s2),
		fmt.Sprintf("user: "+c.s3, p.Name),
		fmt.Sprintf("assistant: "+c.s4),
	}
}

func (g *convGen) pickPersona() persona {
	return g.personas[g.rng.Intn(len(g.personas))]
}

func (g *convGen) pickTopic() string {
	if len(g.topics) == 0 {
		return "事情"
	}
	return g.topics[g.rng.Intn(len(g.topics))]
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func hashString(s string) uint32 {
	var h uint32 = 2166136261
	for i := range len(s) {
		h ^= uint32(s[i])
		h *= 16777619
	}
	return h
}

func toInt(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case float64:
		return int(n), true
	case int64:
		return int(n), true
	}
	return 0, false
}
