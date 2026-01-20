// Doubao TTS Voice List
//
// List all known TTS voices
//
// Usage:
//
//	go run main.go
package main

import (
	"fmt"
)

// Known voices from documentation
// TTS 1.0 voices: https://www.volcengine.com/docs/6561/97465
// TTS 2.0 voices: https://www.volcengine.com/docs/6561/1257544
// Realtime voices: use suffix _jupiter_bigtts

type Voice struct {
	ID          string
	Name        string
	Language    string
	Gender      string
	Description string
	Cluster     string // volcano_tts, volcano_mega, volcano_icl
}

// TTS 1.0 Standard Voices (volcano_tts cluster)
var tts1Voices = []Voice{
	// Chinese Female
	{"BV001_streaming", "通用女声", "zh-CN", "female", "通用场景", "volcano_tts"},
	{"BV002_streaming", "通用男声", "zh-CN", "male", "通用场景", "volcano_tts"},
	{"BV700_streaming", "灿灿", "zh-CN", "female", "甜美活泼", "volcano_tts"},
	{"BV701_streaming", "超自然女声", "zh-CN", "female", "自然流畅", "volcano_tts"},
	{"BV705_streaming", "超自然男声", "zh-CN", "male", "自然流畅", "volcano_tts"},

	// Chinese Dialect
	{"BV021_streaming", "四川话女声", "zh-sichuan", "female", "四川方言", "volcano_tts"},
	{"BV213_streaming", "东北话男声", "zh-dongbei", "male", "东北方言", "volcano_tts"},
	{"BV025_streaming", "台湾女声", "zh-TW", "female", "台湾口音", "volcano_tts"},

	// English
	{"BV503_streaming", "英文女声", "en-US", "female", "美式英语", "volcano_tts"},
	{"BV504_streaming", "英文男声", "en-US", "male", "美式英语", "volcano_tts"},
}

// TTS 2.0 BigModel Voices (volcano_mega cluster)
// Note: These are used via Realtime API with suffix _jupiter_bigtts
var tts2Voices = []Voice{
	// Chinese Female - Standard
	{"zh_female_cancan", "灿灿", "zh-CN", "female", "甜美活泼", "volcano_mega"},
	{"zh_female_shuangshuan", "爽爽", "zh-CN", "female", "知性温柔", "volcano_mega"},
	{"zh_female_qingxin", "清新", "zh-CN", "female", "清新自然", "volcano_mega"},
	{"zh_female_tianmei", "甜美", "zh-CN", "female", "温柔甜美", "volcano_mega"},

	// Chinese Male - Standard
	{"zh_male_yangguang", "阳光", "zh-CN", "male", "阳光活力", "volcano_mega"},
	{"zh_male_wenzhong", "稳重", "zh-CN", "male", "成熟稳重", "volcano_mega"},
	{"zh_male_qingsong", "轻松", "zh-CN", "male", "轻松随和", "volcano_mega"},

	// English
	{"en_female_sweet", "Sweet", "en-US", "female", "甜美英音", "volcano_mega"},
	{"en_male_warm", "Warm", "en-US", "male", "温暖男声", "volcano_mega"},

	// Multi-language
	{"ja_female_warm", "温柔日语女声", "ja", "female", "日语女声", "volcano_mega"},
	{"ko_female_sweet", "甜美韩语女声", "ko", "female", "韩语女声", "volcano_mega"},
}

// Realtime API Voices (volc.speech.dialog)
// These use the bigtts suffix
var realtimeVoices = []Voice{
	{"zh_female_cancan_jupiter_bigtts", "灿灿(实时)", "zh-CN", "female", "甜美活泼-实时对话", "realtime"},
	{"zh_female_qingxin_moon_bigtts", "清新(实时)", "zh-CN", "female", "清新自然-实时对话", "realtime"},
	{"zh_female_shuangkuaisisi_moon_bigtts", "爽快思思(实时)", "zh-CN", "female", "爽快活泼-实时对话", "realtime"},
	{"BV700_streaming_jupiter_bigtts", "灿灿V1(实时)", "zh-CN", "female", "甜美活泼-实时对话", "realtime"},
}

func main() {
	fmt.Println("=== Doubao TTS Voice List ===")
	fmt.Println("")

	fmt.Println("📌 TTS 1.0 Standard Voices (volcano_tts)")
	fmt.Println("   Required service: volc.tts.default")
	fmt.Println("   ─────────────────────────────────────────────")
	for _, v := range tts1Voices {
		fmt.Printf("   %-25s %-10s %-10s %s\n", v.ID, v.Name, v.Gender, v.Language)
	}

	fmt.Println("")
	fmt.Println("📌 TTS 2.0 BigModel Voices (volcano_mega)")
	fmt.Println("   Required service: volc.seedtts.default")
	fmt.Println("   ─────────────────────────────────────────────")
	for _, v := range tts2Voices {
		fmt.Printf("   %-25s %-10s %-10s %s\n", v.ID, v.Name, v.Gender, v.Language)
	}

	fmt.Println("")
	fmt.Println("📌 Realtime Voices (volc.speech.dialog)")
	fmt.Println("   Use with Realtime API, voice ID has _jupiter_bigtts suffix")
	fmt.Println("   ✅ This service is enabled!")
	fmt.Println("   ─────────────────────────────────────────────")
	for _, v := range realtimeVoices {
		fmt.Printf("   %-40s %-15s %s\n", v.ID, v.Name, v.Language)
	}

	fmt.Println("")
	fmt.Println("🔗 More voices available at:")
	fmt.Println("   TTS 1.0: https://www.volcengine.com/docs/6561/97465")
	fmt.Println("   TTS 2.0: https://www.volcengine.com/docs/6561/1257544")

	fmt.Println("")
	fmt.Println("💡 Usage Notes:")
	fmt.Println("   - TTS 1.0/2.0 requires enabling the service in Volcengine console")
	fmt.Println("   - Realtime voices use volc.speech.dialog service")
	fmt.Println("   - Custom cloned voices use volcano_icl cluster, Format: S_xxx or ICL_xxx")
}
