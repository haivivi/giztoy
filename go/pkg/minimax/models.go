package minimax

// 文本模型
const (
	// ModelM2_1 MiniMax-M2.1，强大多语言编程能力，输出速度约 60 tps
	ModelM2_1 = "MiniMax-M2.1"

	// ModelM2_1Lightning MiniMax-M2.1-lightning，M2.1 极速版，输出速度约 100 tps
	ModelM2_1Lightning = "MiniMax-M2.1-lightning"

	// ModelM2 MiniMax-M2，专为高效编码与 Agent 工作流而生
	ModelM2 = "MiniMax-M2"
)

// 语音模型
const (
	// ModelSpeech26HD speech-2.6-hd，最新 HD 模型，韵律表现出色
	ModelSpeech26HD = "speech-2.6-hd"

	// ModelSpeech26Turbo speech-2.6-turbo，最新 Turbo 模型，超低时延
	ModelSpeech26Turbo = "speech-2.6-turbo"

	// ModelSpeech02HD speech-02-hd，出色的韵律、稳定性和复刻相似度
	ModelSpeech02HD = "speech-02-hd"

	// ModelSpeech02Turbo speech-02-turbo，小语种能力加强
	ModelSpeech02Turbo = "speech-02-turbo"
)

// 视频模型
const (
	// ModelHailuo23 MiniMax-Hailuo-2.3，最新视频模型
	ModelHailuo23 = "MiniMax-Hailuo-2.3"

	// ModelHailuo23Fast MiniMax-Hailuo-2.3-Fast，快速版本
	ModelHailuo23Fast = "MiniMax-Hailuo-2.3-Fast"

	// ModelHailuo02 MiniMax-Hailuo-02，支持 1080P 和 10 秒视频
	ModelHailuo02 = "MiniMax-Hailuo-02"

	// ModelT2V01 T2V-01，文生视频模型
	ModelT2V01 = "T2V-01"

	// ModelT2V01Director T2V-01-Director，支持镜头运动控制
	ModelT2V01Director = "T2V-01-Director"

	// ModelI2V01 I2V-01，图生视频模型
	ModelI2V01 = "I2V-01"

	// ModelI2V01Director I2V-01-Director，图生视频，支持镜头控制
	ModelI2V01Director = "I2V-01-Director"

	// ModelI2V01Live I2V-01-live，图生视频，支持多种画风
	ModelI2V01Live = "I2V-01-live"
)

// 图片模型
const (
	// ModelImage01 image-01，图像生成模型，画面表现细腻
	ModelImage01 = "image-01"

	// ModelImage01Live image-01-live，额外支持多种画风设置
	ModelImage01Live = "image-01-live"
)

// 音乐模型
const (
	// ModelMusic20 music-2.0，最新音乐生成模型
	ModelMusic20 = "music-2.0"
)

// 常用音色 ID
const (
	// VoiceMaleQingse 青涩青年音
	VoiceMaleQingse = "male-qn-qingse"

	// VoiceMaleJingying 精英青年音
	VoiceMaleJingying = "male-qn-jingying"

	// VoiceMaleBadao 霸道青年音
	VoiceMaleBadao = "male-qn-badao"

	// VoiceFemaleShaonv 少女音
	VoiceFemaleShaonv = "female-shaonv"

	// VoiceFemaleYujie 御姐音
	VoiceFemaleYujie = "female-yujie"

	// VoiceFemaleChengshu 成熟女性音
	VoiceFemaleChengshu = "female-chengshu"

	// VoicePresenterMale 男性播音员
	VoicePresenterMale = "presenter_male"

	// VoicePresenterFemale 女性播音员
	VoicePresenterFemale = "presenter_female"

	// VoiceAudiobookMale1 有声书男声1
	VoiceAudiobookMale1 = "audiobook_male_1"

	// VoiceAudiobookFemale1 有声书女声1
	VoiceAudiobookFemale1 = "audiobook_female_1"

	// VoiceCuteBoy 可爱男孩
	VoiceCuteBoy = "cute_boy"

	// VoiceCharmingLady 魅力女性
	VoiceCharmingLady = "Charming_Lady"
)

// 语言增强选项
const (
	LanguageChinese    = "Chinese"
	LanguageChineseYue = "Chinese,Yue" // 粤语
	LanguageEnglish    = "English"
	LanguageJapanese   = "Japanese"
	LanguageKorean     = "Korean"
	LanguageFrench     = "French"
	LanguageGerman     = "German"
	LanguageSpanish    = "Spanish"
	LanguageItalian    = "Italian"
	LanguagePortuguese = "Portuguese"
	LanguageRussian    = "Russian"
	LanguageArabic     = "Arabic"
	LanguageThai       = "Thai"
	LanguageVietnamese = "Vietnamese"
	LanguageIndonesian = "Indonesian"
	LanguageAuto       = "auto"
)

// 情绪选项
const (
	EmotionHappy     = "happy"
	EmotionSad       = "sad"
	EmotionAngry     = "angry"
	EmotionFearful   = "fearful"
	EmotionDisgusted = "disgusted"
	EmotionSurprised = "surprised"
	EmotionNeutral   = "neutral"
)

// 图片比例
const (
	AspectRatio1x1  = "1:1"
	AspectRatio16x9 = "16:9"
	AspectRatio9x16 = "9:16"
	AspectRatio4x3  = "4:3"
	AspectRatio3x4  = "3:4"
	AspectRatio3x2  = "3:2"
	AspectRatio2x3  = "2:3"
	AspectRatio21x9 = "21:9"
	AspectRatio9x21 = "9:21"
)

// 视频分辨率
const (
	Resolution768P  = "768P"
	Resolution1080P = "1080P"
)
