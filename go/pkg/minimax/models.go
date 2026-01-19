package minimax

// Text models
const (
	// ModelM2_1 is MiniMax-M2.1, powerful multi-language programming, ~60 tps output.
	ModelM2_1 = "MiniMax-M2.1"

	// ModelM2_1Lightning is MiniMax-M2.1-lightning, faster version, ~100 tps output.
	ModelM2_1Lightning = "MiniMax-M2.1-lightning"

	// ModelM2 is MiniMax-M2, designed for efficient coding and agent workflows.
	ModelM2 = "MiniMax-M2"
)

// Speech models
const (
	// ModelSpeech26HD is speech-2.6-hd, latest HD model with outstanding prosody.
	ModelSpeech26HD = "speech-2.6-hd"

	// ModelSpeech26Turbo is speech-2.6-turbo, latest Turbo model with ultra-low latency.
	ModelSpeech26Turbo = "speech-2.6-turbo"

	// ModelSpeech02HD is speech-02-hd, excellent rhythm, stability and cloning similarity.
	ModelSpeech02HD = "speech-02-hd"

	// ModelSpeech02Turbo is speech-02-turbo, enhanced multilingual capabilities.
	ModelSpeech02Turbo = "speech-02-turbo"
)

// Video models
const (
	// ModelHailuo23 is MiniMax-Hailuo-2.3, latest video generation model.
	ModelHailuo23 = "MiniMax-Hailuo-2.3"

	// ModelHailuo23Fast is MiniMax-Hailuo-2.3-Fast, faster version.
	ModelHailuo23Fast = "MiniMax-Hailuo-2.3-Fast"

	// ModelHailuo02 is MiniMax-Hailuo-02, supports 1080P and 10s videos.
	ModelHailuo02 = "MiniMax-Hailuo-02"

	// ModelT2V01 is T2V-01, text-to-video model.
	ModelT2V01 = "T2V-01"

	// ModelT2V01Director is T2V-01-Director, supports camera movement control.
	ModelT2V01Director = "T2V-01-Director"

	// ModelI2V01 is I2V-01, image-to-video model.
	ModelI2V01 = "I2V-01"

	// ModelI2V01Director is I2V-01-Director, image-to-video with camera control.
	ModelI2V01Director = "I2V-01-Director"

	// ModelI2V01Live is I2V-01-live, image-to-video with multiple styles.
	ModelI2V01Live = "I2V-01-live"
)

// Image models
const (
	// ModelImage01 is image-01, image generation model with fine-grained details.
	ModelImage01 = "image-01"

	// ModelImage01Live is image-01-live, supports multiple style settings.
	ModelImage01Live = "image-01-live"
)

// Music models
const (
	// ModelMusic20 is music-2.0, latest music generation model.
	ModelMusic20 = "music-2.0"
)

// Common voice IDs
const (
	// VoiceMaleQingse is a young male voice (Chinese).
	VoiceMaleQingse = "male-qn-qingse"

	// VoiceMaleJingying is an elite young male voice (Chinese).
	VoiceMaleJingying = "male-qn-jingying"

	// VoiceMaleBadao is a domineering young male voice (Chinese).
	VoiceMaleBadao = "male-qn-badao"

	// VoiceFemaleShaonv is a young girl voice (Chinese).
	VoiceFemaleShaonv = "female-shaonv"

	// VoiceFemaleYujie is a mature female voice (Chinese).
	VoiceFemaleYujie = "female-yujie"

	// VoiceFemaleChengshu is a mature woman voice (Chinese).
	VoiceFemaleChengshu = "female-chengshu"

	// VoicePresenterMale is a male presenter voice.
	VoicePresenterMale = "presenter_male"

	// VoicePresenterFemale is a female presenter voice.
	VoicePresenterFemale = "presenter_female"

	// VoiceAudiobookMale1 is an audiobook male voice.
	VoiceAudiobookMale1 = "audiobook_male_1"

	// VoiceAudiobookFemale1 is an audiobook female voice.
	VoiceAudiobookFemale1 = "audiobook_female_1"

	// VoiceCuteBoy is a cute boy voice.
	VoiceCuteBoy = "cute_boy"

	// VoiceCharmingLady is a charming lady voice.
	VoiceCharmingLady = "Charming_Lady"
)

// Language boost options
const (
	LanguageChinese    = "Chinese"
	LanguageChineseYue = "Chinese,Yue" // Cantonese
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

// Emotion options
const (
	EmotionHappy     = "happy"
	EmotionSad       = "sad"
	EmotionAngry     = "angry"
	EmotionFearful   = "fearful"
	EmotionDisgusted = "disgusted"
	EmotionSurprised = "surprised"
	EmotionNeutral   = "neutral"
)

// Image aspect ratios
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

// Video resolutions
const (
	Resolution768P  = "768P"
	Resolution1080P = "1080P"
)
