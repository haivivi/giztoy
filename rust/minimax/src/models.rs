//! Model constants and predefined values for MiniMax API.

// ==================== Text Models ====================

/// MiniMax-M2.1, powerful multi-language programming, ~60 tps output.
pub const MODEL_M2_1: &str = "MiniMax-M2.1";

/// MiniMax-M2.1-lightning, faster version, ~100 tps output.
pub const MODEL_M2_1_LIGHTNING: &str = "MiniMax-M2.1-lightning";

/// MiniMax-M2, designed for efficient coding and agent workflows.
pub const MODEL_M2: &str = "MiniMax-M2";

// ==================== Speech Models ====================

/// speech-2.6-hd, latest HD model with outstanding prosody.
pub const MODEL_SPEECH_26_HD: &str = "speech-2.6-hd";

/// speech-2.6-turbo, latest Turbo model with ultra-low latency.
pub const MODEL_SPEECH_26_TURBO: &str = "speech-2.6-turbo";

/// speech-02-hd, excellent rhythm, stability and cloning similarity.
pub const MODEL_SPEECH_02_HD: &str = "speech-02-hd";

/// speech-02-turbo, enhanced multilingual capabilities.
pub const MODEL_SPEECH_02_TURBO: &str = "speech-02-turbo";

// ==================== Video Models ====================

/// MiniMax-Hailuo-2.3, latest video generation model.
pub const MODEL_HAILUO_23: &str = "MiniMax-Hailuo-2.3";

/// MiniMax-Hailuo-2.3-Fast, faster version.
pub const MODEL_HAILUO_23_FAST: &str = "MiniMax-Hailuo-2.3-Fast";

/// MiniMax-Hailuo-02, supports 1080P and 10s videos.
pub const MODEL_HAILUO_02: &str = "MiniMax-Hailuo-02";

/// T2V-01, text-to-video model.
pub const MODEL_T2V_01: &str = "T2V-01";

/// T2V-01-Director, supports camera movement control.
pub const MODEL_T2V_01_DIRECTOR: &str = "T2V-01-Director";

/// I2V-01, image-to-video model.
pub const MODEL_I2V_01: &str = "I2V-01";

/// I2V-01-Director, image-to-video with camera control.
pub const MODEL_I2V_01_DIRECTOR: &str = "I2V-01-Director";

/// I2V-01-live, image-to-video with multiple styles.
pub const MODEL_I2V_01_LIVE: &str = "I2V-01-live";

// ==================== Image Models ====================

/// image-01, image generation model with fine-grained details.
pub const MODEL_IMAGE_01: &str = "image-01";

/// image-01-live, supports multiple style settings.
pub const MODEL_IMAGE_01_LIVE: &str = "image-01-live";

// ==================== Music Models ====================

/// music-2.0, latest music generation model.
pub const MODEL_MUSIC_20: &str = "music-2.0";

// ==================== Voice IDs ====================

/// A young male voice (Chinese).
pub const VOICE_MALE_QINGSE: &str = "male-qn-qingse";

/// An elite young male voice (Chinese).
pub const VOICE_MALE_JINGYING: &str = "male-qn-jingying";

/// A domineering young male voice (Chinese).
pub const VOICE_MALE_BADAO: &str = "male-qn-badao";

/// A young girl voice (Chinese).
pub const VOICE_FEMALE_SHAONV: &str = "female-shaonv";

/// A mature female voice (Chinese).
pub const VOICE_FEMALE_YUJIE: &str = "female-yujie";

/// A mature woman voice (Chinese).
pub const VOICE_FEMALE_CHENGSHU: &str = "female-chengshu";

/// A male presenter voice.
pub const VOICE_PRESENTER_MALE: &str = "presenter_male";

/// A female presenter voice.
pub const VOICE_PRESENTER_FEMALE: &str = "presenter_female";

/// An audiobook male voice.
pub const VOICE_AUDIOBOOK_MALE_1: &str = "audiobook_male_1";

/// An audiobook female voice.
pub const VOICE_AUDIOBOOK_FEMALE_1: &str = "audiobook_female_1";

/// A cute boy voice.
pub const VOICE_CUTE_BOY: &str = "cute_boy";

/// A charming lady voice.
pub const VOICE_CHARMING_LADY: &str = "Charming_Lady";

// ==================== Language Boost Options ====================

pub const LANGUAGE_CHINESE: &str = "Chinese";
pub const LANGUAGE_CHINESE_YUE: &str = "Chinese,Yue"; // Cantonese
pub const LANGUAGE_ENGLISH: &str = "English";
pub const LANGUAGE_JAPANESE: &str = "Japanese";
pub const LANGUAGE_KOREAN: &str = "Korean";
pub const LANGUAGE_FRENCH: &str = "French";
pub const LANGUAGE_GERMAN: &str = "German";
pub const LANGUAGE_SPANISH: &str = "Spanish";
pub const LANGUAGE_ITALIAN: &str = "Italian";
pub const LANGUAGE_PORTUGUESE: &str = "Portuguese";
pub const LANGUAGE_RUSSIAN: &str = "Russian";
pub const LANGUAGE_ARABIC: &str = "Arabic";
pub const LANGUAGE_THAI: &str = "Thai";
pub const LANGUAGE_VIETNAMESE: &str = "Vietnamese";
pub const LANGUAGE_INDONESIAN: &str = "Indonesian";
pub const LANGUAGE_AUTO: &str = "auto";

// ==================== Emotion Options ====================

pub const EMOTION_HAPPY: &str = "happy";
pub const EMOTION_SAD: &str = "sad";
pub const EMOTION_ANGRY: &str = "angry";
pub const EMOTION_FEARFUL: &str = "fearful";
pub const EMOTION_DISGUSTED: &str = "disgusted";
pub const EMOTION_SURPRISED: &str = "surprised";
pub const EMOTION_NEUTRAL: &str = "neutral";

// ==================== Image Aspect Ratios ====================

pub const ASPECT_RATIO_1X1: &str = "1:1";
pub const ASPECT_RATIO_16X9: &str = "16:9";
pub const ASPECT_RATIO_9X16: &str = "9:16";
pub const ASPECT_RATIO_4X3: &str = "4:3";
pub const ASPECT_RATIO_3X4: &str = "3:4";
pub const ASPECT_RATIO_3X2: &str = "3:2";
pub const ASPECT_RATIO_2X3: &str = "2:3";
pub const ASPECT_RATIO_21X9: &str = "21:9";
pub const ASPECT_RATIO_9X21: &str = "9:21";

// ==================== Video Resolutions ====================

pub const RESOLUTION_768P: &str = "768P";
pub const RESOLUTION_1080P: &str = "1080P";
