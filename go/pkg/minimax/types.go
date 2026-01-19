package minimax

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// FlexibleID is a custom type that can unmarshal both string and number JSON values.
// This is needed because some MiniMax APIs return file_id as int64 while others as string.
type FlexibleID string

// UnmarshalJSON implements json.Unmarshaler interface.
func (f *FlexibleID) UnmarshalJSON(data []byte) error {
	// Try to unmarshal as string first
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		*f = FlexibleID(s)
		return nil
	}

	// Try to unmarshal as number
	var n int64
	if err := json.Unmarshal(data, &n); err == nil {
		*f = FlexibleID(strconv.FormatInt(n, 10))
		return nil
	}

	return fmt.Errorf("FlexibleID: cannot unmarshal %s", string(data))
}

// MarshalJSON implements json.Marshaler interface.
func (f FlexibleID) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(f))
}

// String returns the string representation of the ID.
func (f FlexibleID) String() string {
	return string(f)
}

// ================== Common Types ==================

// OutputFormat specifies the output format for audio.
type OutputFormat string

const (
	OutputFormatHex OutputFormat = "hex"
	OutputFormatURL OutputFormat = "url"
)

// AudioFormat specifies the audio encoding format.
type AudioFormat string

const (
	AudioFormatMP3  AudioFormat = "mp3"
	AudioFormatPCM  AudioFormat = "pcm"
	AudioFormatFLAC AudioFormat = "flac"
	AudioFormatWAV  AudioFormat = "wav"
)

// VoiceType specifies the type of voice to filter.
type VoiceType string

const (
	VoiceTypeAll        VoiceType = "all"
	VoiceTypeSystem     VoiceType = "system"
	VoiceTypeCloning    VoiceType = "voice_cloning"
	VoiceTypeGeneration VoiceType = "voice_generation"
)

// FilePurpose specifies the intended use of an uploaded file.
type FilePurpose string

const (
	// FilePurposeVoiceClone is for voice cloning source audio.
	FilePurposeVoiceClone FilePurpose = "voice_clone"

	// FilePurposePromptAudio is for voice cloning example/prompt audio.
	FilePurposePromptAudio FilePurpose = "prompt_audio"

	// FilePurposeT2AAsyncInput is for async TTS input files.
	FilePurposeT2AAsyncInput FilePurpose = "t2a_async_input"
)

// TaskStatus represents the status of an async task.
type TaskStatus string

const (
	TaskStatusPending    TaskStatus = "Pending"
	TaskStatusQueueing   TaskStatus = "Queueing"
	TaskStatusPreparing  TaskStatus = "Preparing"
	TaskStatusProcessing TaskStatus = "Processing"
	TaskStatusSuccess    TaskStatus = "Success"
	TaskStatusFailed     TaskStatus = "Failed"
)

// ================== Audio Types ==================

// AudioInfo contains metadata about generated audio.
type AudioInfo struct {
	// AudioLength is the duration in milliseconds.
	AudioLength int `json:"audio_length"`

	// AudioSampleRate is the sample rate.
	AudioSampleRate int `json:"audio_sample_rate"`

	// AudioSize is the size in bytes.
	AudioSize int `json:"audio_size"`

	// Bitrate is the bitrate.
	Bitrate int `json:"bitrate"`

	// WordCount is the number of words/characters.
	WordCount int `json:"word_count"`

	// UsageCharacters is the billable character count.
	UsageCharacters int `json:"usage_characters"`

	// AudioFormat is the audio format.
	AudioFormat string `json:"audio_format"`

	// AudioChannel is the number of channels.
	AudioChannel int `json:"audio_channel"`
}

// Subtitle contains subtitle information.
type Subtitle struct {
	Segments []SubtitleSegment `json:"segments"`
}

// SubtitleSegment represents a single subtitle segment.
type SubtitleSegment struct {
	StartTime int    `json:"start_time"` // milliseconds
	EndTime   int    `json:"end_time"`   // milliseconds
	Text      string `json:"text"`
}

// ================== Speech Types ==================

// SpeechRequest is the request for speech synthesis.
type SpeechRequest struct {
	// Model is the model version.
	Model string `json:"model" yaml:"model"`

	// Text is the text to synthesize (max 10,000 characters).
	Text string `json:"text" yaml:"text"`

	// VoiceSetting contains voice configuration.
	VoiceSetting *VoiceSetting `json:"voice_setting,omitempty" yaml:"voice_setting,omitempty"`

	// AudioSetting contains audio configuration.
	AudioSetting *AudioSetting `json:"audio_setting,omitempty" yaml:"audio_setting,omitempty"`

	// PronunciationDict contains pronunciation rules.
	PronunciationDict *PronunciationDict `json:"pronunciation_dict,omitempty" yaml:"pronunciation_dict,omitempty"`

	// LanguageBoost enhances specific language pronunciation.
	LanguageBoost string `json:"language_boost,omitempty" yaml:"language_boost,omitempty"`

	// SubtitleEnable enables subtitle generation.
	SubtitleEnable bool `json:"subtitle_enable,omitempty" yaml:"subtitle_enable,omitempty"`

	// OutputFormat specifies output format: hex or url.
	OutputFormat OutputFormat `json:"output_format,omitempty" yaml:"output_format,omitempty"`
}

// AsyncSpeechRequest is the request for async speech synthesis.
type AsyncSpeechRequest struct {
	// Model is the model version.
	Model string `json:"model"`

	// Text is the text to synthesize (max 1,000,000 characters).
	Text string `json:"text,omitempty"`

	// FileID is the file_id of a text file (alternative to Text).
	FileID string `json:"file_id,omitempty"`

	// VoiceSetting contains voice configuration.
	VoiceSetting *VoiceSetting `json:"voice_setting,omitempty"`

	// AudioSetting contains audio configuration.
	AudioSetting *AudioSetting `json:"audio_setting,omitempty"`

	// PronunciationDict contains pronunciation rules.
	PronunciationDict *PronunciationDict `json:"pronunciation_dict,omitempty"`

	// LanguageBoost enhances specific language pronunciation.
	LanguageBoost string `json:"language_boost,omitempty"`

	// SubtitleEnable enables subtitle generation.
	SubtitleEnable bool `json:"subtitle_enable,omitempty"`
}

// VoiceSetting contains voice configuration.
type VoiceSetting struct {
	// VoiceID is the voice identifier.
	VoiceID string `json:"voice_id" yaml:"voice_id"`

	// Speed is the speech speed (0.5-2.0, default 1.0).
	Speed float64 `json:"speed,omitempty" yaml:"speed,omitempty"`

	// Vol is the volume (0-10, default 1.0).
	Vol float64 `json:"vol,omitempty" yaml:"vol,omitempty"`

	// Pitch is the pitch adjustment (-12 to 12, default 0).
	Pitch int `json:"pitch,omitempty" yaml:"pitch,omitempty"`

	// Emotion is the emotion: happy, sad, angry, fearful, disgusted, surprised, neutral.
	Emotion string `json:"emotion,omitempty" yaml:"emotion,omitempty"`
}

// AudioSetting contains audio configuration.
type AudioSetting struct {
	// SampleRate is the sample rate: 8000, 16000, 22050, 24000, 32000, 44100.
	SampleRate int `json:"sample_rate,omitempty" yaml:"sample_rate,omitempty"`

	// Bitrate is the bitrate: 32000, 64000, 128000, 256000.
	Bitrate int `json:"bitrate,omitempty" yaml:"bitrate,omitempty"`

	// Format is the audio format: mp3, pcm, flac, wav.
	Format AudioFormat `json:"format,omitempty" yaml:"format,omitempty"`

	// Channel is the number of channels: 1 or 2.
	Channel int `json:"channel,omitempty" yaml:"channel,omitempty"`
}

// PronunciationDict contains pronunciation rules.
type PronunciationDict struct {
	// Tone is a list of pronunciation rules, e.g. ["处理/(chu3)(li3)", "危险/dangerous"].
	Tone []string `json:"tone"`
}

// SpeechResponse is the response from speech synthesis.
type SpeechResponse struct {
	// Audio is the decoded audio data.
	Audio []byte `json:"-"`

	// AudioURL is the audio URL (when OutputFormat is "url").
	AudioURL string `json:"audio_url,omitempty"`

	// ExtraInfo contains audio metadata.
	ExtraInfo *AudioInfo `json:"extra_info"`

	// TraceID is the request trace ID.
	TraceID string `json:"trace_id"`
}

// SpeechChunk represents a chunk of streaming speech data.
type SpeechChunk struct {
	// Audio is the decoded audio data (may be nil).
	Audio []byte

	// Status is the status code: 1=generating, 2=complete.
	Status int `json:"status"`

	// Subtitle is the subtitle segment (if enabled).
	Subtitle *SubtitleSegment `json:"subtitle,omitempty"`

	// ExtraInfo contains audio metadata (usually in last chunk).
	ExtraInfo *AudioInfo `json:"extra_info,omitempty"`

	// TraceID is the request trace ID (usually in last chunk).
	TraceID string `json:"trace_id,omitempty"`
}

// SpeechAsyncResult is the result of an async speech task.
type SpeechAsyncResult struct {
	// FileID is the generated audio file ID.
	FileID string `json:"file_id"`

	// AudioInfo contains audio metadata.
	AudioInfo *AudioInfo `json:"extra_info"`

	// Subtitle contains subtitle information (if enabled).
	Subtitle *Subtitle `json:"subtitle,omitempty"`
}

// ================== Text Types ==================

// ChatCompletionRequest is the request for chat completion.
type ChatCompletionRequest struct {
	// Model is the model name.
	Model string `json:"model" yaml:"model"`

	// Messages is the conversation history.
	Messages []Message `json:"messages" yaml:"messages"`

	// MaxTokens is the maximum output tokens.
	MaxTokens int `json:"max_tokens,omitempty" yaml:"max_tokens,omitempty"`

	// Temperature is the sampling temperature (0-2).
	Temperature float64 `json:"temperature,omitempty" yaml:"temperature,omitempty"`

	// TopP is the nucleus sampling parameter.
	TopP float64 `json:"top_p,omitempty" yaml:"top_p,omitempty"`

	// Tools is the list of available tools.
	Tools []Tool `json:"tools,omitempty" yaml:"tools,omitempty"`

	// ToolChoice is the tool selection strategy.
	ToolChoice any `json:"tool_choice,omitempty" yaml:"tool_choice,omitempty"`
}

// Message represents a chat message.
type Message struct {
	// Role is the message role: system, user, assistant.
	Role string `json:"role" yaml:"role"`

	// Content is the message content (string or content array).
	Content any `json:"content" yaml:"content"`

	// ToolCalls contains tool calls (for assistant messages).
	ToolCalls []ToolCall `json:"tool_calls,omitempty" yaml:"tool_calls,omitempty"`

	// ToolCallID is the tool call ID (for tool messages).
	ToolCallID string `json:"tool_call_id,omitempty" yaml:"tool_call_id,omitempty"`
}

// Tool represents a tool definition.
type Tool struct {
	Type     string       `json:"type"`
	Function FunctionTool `json:"function"`
}

// FunctionTool represents a function tool definition.
type FunctionTool struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Parameters  any    `json:"parameters,omitempty"`
}

// ToolCall represents a tool call.
type ToolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"`
	Function FunctionToolCall `json:"function"`
}

// FunctionToolCall represents a function tool call.
type FunctionToolCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ChatCompletionResponse is the response from chat completion.
type ChatCompletionResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   *Usage   `json:"usage"`
}

// Choice represents a completion choice.
type Choice struct {
	Index        int      `json:"index"`
	Message      *Message `json:"message"`
	FinishReason string   `json:"finish_reason"`
}

// Usage contains token usage information.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ChatCompletionChunk represents a streaming chat chunk.
type ChatCompletionChunk struct {
	ID      string        `json:"id"`
	Object  string        `json:"object"`
	Created int64         `json:"created"`
	Model   string        `json:"model"`
	Choices []ChunkChoice `json:"choices"`
}

// ChunkChoice represents a streaming choice.
type ChunkChoice struct {
	Index        int         `json:"index"`
	Delta        *ChunkDelta `json:"delta"`
	FinishReason string      `json:"finish_reason,omitempty"`
}

// ChunkDelta represents the delta content in a streaming chunk.
type ChunkDelta struct {
	Role      string     `json:"role,omitempty"`
	Content   string     `json:"content,omitempty"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
}

// ================== Voice Types ==================

// VoiceListResponse is the response containing available voices.
type VoiceListResponse struct {
	// SystemVoices contains system predefined voices.
	SystemVoices []VoiceInfo `json:"system_voice"`

	// CloningVoices contains voices created via voice cloning.
	CloningVoices []VoiceInfo `json:"voice_cloning"`

	// GenerationVoices contains voices created via voice design/generation.
	GenerationVoices []VoiceInfo `json:"voice_generation"`
}

// AllVoices returns all voices combined into a single slice with Type field set.
func (r *VoiceListResponse) AllVoices() []VoiceInfo {
	all := make([]VoiceInfo, 0, len(r.SystemVoices)+len(r.CloningVoices)+len(r.GenerationVoices))
	for i := range r.SystemVoices {
		voice := r.SystemVoices[i]
		voice.Type = "system"
		all = append(all, voice)
	}
	for i := range r.CloningVoices {
		voice := r.CloningVoices[i]
		voice.Type = "voice_cloning"
		all = append(all, voice)
	}
	for i := range r.GenerationVoices {
		voice := r.GenerationVoices[i]
		voice.Type = "voice_generation"
		all = append(all, voice)
	}
	return all
}

// VoiceInfo contains information about a voice.
type VoiceInfo struct {
	// VoiceID is the voice identifier.
	VoiceID string `json:"voice_id"`

	// VoiceName is the voice name (from API response).
	VoiceName string `json:"voice_name"`

	// Type is the voice type: system, voice_cloning, voice_generation.
	Type string `json:"type,omitempty"`

	// Description is the voice description (array from API).
	Description []string `json:"description,omitempty"`

	// CreatedTime is the creation time.
	CreatedTime string `json:"created_time,omitempty"`
}

// UploadResponse is the response from file upload.
type UploadResponse struct {
	FileID string `json:"file_id"`
}

// VoiceCloneRequest is the request for voice cloning.
type VoiceCloneRequest struct {
	// FileID is the file_id of the clone audio (must be int64).
	FileID int64 `json:"file_id" yaml:"file_id"`

	// DemoFileID is the file_id of the demo audio (optional).
	DemoFileID int64 `json:"demo_file_id,omitempty" yaml:"demo_file_id,omitempty"`

	// VoiceID is the custom voice ID.
	VoiceID string `json:"voice_id" yaml:"voice_id"`

	// Model is the model version.
	Model string `json:"model,omitempty" yaml:"model,omitempty"`

	// Text is the preview text.
	Text string `json:"text,omitempty" yaml:"text,omitempty"`
}

// VoiceCloneResponse is the response from voice cloning.
type VoiceCloneResponse struct {
	// VoiceID is the cloned voice ID.
	VoiceID string `json:"voice_id"`

	// DemoAudio is the decoded demo audio.
	DemoAudio []byte `json:"-"`
}

// VoiceDesignRequest is the request for voice design.
type VoiceDesignRequest struct {
	// Prompt is the voice description.
	Prompt string `json:"prompt" yaml:"prompt"`

	// PreviewText is the preview text.
	PreviewText string `json:"preview_text" yaml:"preview_text"`

	// VoiceID is the custom voice ID (optional).
	VoiceID string `json:"voice_id,omitempty" yaml:"voice_id,omitempty"`

	// Model is the model version.
	Model string `json:"model,omitempty" yaml:"model,omitempty"`
}

// VoiceDesignResponse is the response from voice design.
type VoiceDesignResponse struct {
	// VoiceID is the designed voice ID.
	VoiceID string `json:"voice_id"`

	// DemoAudio is the decoded demo audio.
	DemoAudio []byte `json:"-"`
}

// ================== Video Types ==================

// TextToVideoRequest is the request for text-to-video generation.
type TextToVideoRequest struct {
	// Model is the model name.
	Model string `json:"model" yaml:"model"`

	// Prompt is the video description.
	Prompt string `json:"prompt" yaml:"prompt"`

	// Duration is the video duration in seconds (6 or 10).
	Duration int `json:"duration,omitempty" yaml:"duration,omitempty"`

	// Resolution is the resolution: 768P or 1080P.
	Resolution string `json:"resolution,omitempty" yaml:"resolution,omitempty"`
}

// ImageToVideoRequest is the request for image-to-video generation.
type ImageToVideoRequest struct {
	// Model is the model name (I2V series).
	Model string `json:"model" yaml:"model"`

	// Prompt is the video description.
	Prompt string `json:"prompt,omitempty" yaml:"prompt,omitempty"`

	// FirstFrameImage is the first frame image URL or base64.
	FirstFrameImage string `json:"first_frame_image" yaml:"first_frame_image"`

	// Duration is the video duration in seconds.
	Duration int `json:"duration,omitempty" yaml:"duration,omitempty"`

	// Resolution is the resolution.
	Resolution string `json:"resolution,omitempty" yaml:"resolution,omitempty"`
}

// FrameToVideoRequest is the request for first/last frame video generation.
type FrameToVideoRequest struct {
	// Model is the model name.
	Model string `json:"model" yaml:"model"`

	// Prompt is the video description.
	Prompt string `json:"prompt,omitempty" yaml:"prompt,omitempty"`

	// FirstFrameImage is the first frame image.
	FirstFrameImage string `json:"first_frame_image" yaml:"first_frame_image"`

	// LastFrameImage is the last frame image.
	LastFrameImage string `json:"last_frame_image" yaml:"last_frame_image"`
}

// SubjectRefVideoRequest is the request for subject reference video generation.
type SubjectRefVideoRequest struct {
	// Model is the model name.
	Model string `json:"model"`

	// Prompt is the video description.
	Prompt string `json:"prompt"`

	// SubjectReference is the subject reference image.
	SubjectReference string `json:"subject_reference"`
}

// VideoAgentRequest is the request for video agent task.
type VideoAgentRequest struct {
	// TemplateID is the template ID.
	TemplateID string `json:"template_id"`

	// MediaInputs contains media inputs.
	MediaInputs []MediaInput `json:"media_inputs,omitempty"`

	// TextInputs contains text inputs.
	TextInputs []TextInput `json:"text_inputs,omitempty"`
}

// MediaInput represents a media input for video agent.
type MediaInput struct {
	// Type is the media type: image or video.
	Type string `json:"type"`

	// URL is the media file URL.
	URL string `json:"url,omitempty"`

	// FileID is the media file ID.
	FileID string `json:"file_id,omitempty"`
}

// TextInput represents a text input for video agent.
type TextInput struct {
	// Key is the input key name.
	Key string `json:"key"`

	// Value is the input value.
	Value string `json:"value"`
}

// VideoResult is the result of a video generation task.
type VideoResult struct {
	// FileID is the generated video file ID.
	FileID string `json:"file_id"`

	// VideoWidth is the width of the generated video.
	VideoWidth int `json:"video_width,omitempty"`

	// VideoHeight is the height of the generated video.
	VideoHeight int `json:"video_height,omitempty"`

	// DownloadURL is the video download URL (for agent tasks).
	DownloadURL string `json:"download_url,omitempty"`
}

// ================== Image Types ==================

// ImageGenerateRequest is the request for image generation.
type ImageGenerateRequest struct {
	// Model is the model name.
	Model string `json:"model" yaml:"model"`

	// Prompt is the image description.
	Prompt string `json:"prompt" yaml:"prompt"`

	// AspectRatio is the aspect ratio: 1:1, 16:9, 9:16, 4:3, 3:4, 3:2, 2:3, 21:9, 9:21.
	AspectRatio string `json:"aspect_ratio,omitempty" yaml:"aspect_ratio,omitempty"`

	// N is the number of images to generate (1-9).
	N int `json:"n,omitempty" yaml:"n,omitempty"`

	// PromptOptimizer enables prompt optimization.
	PromptOptimizer *bool `json:"prompt_optimizer,omitempty" yaml:"prompt_optimizer,omitempty"`
}

// ImageReferenceRequest is the request for image generation with reference.
type ImageReferenceRequest struct {
	ImageGenerateRequest `yaml:",inline"`

	// ImagePrompt is the reference image URL.
	ImagePrompt string `json:"image_prompt" yaml:"image_prompt"`

	// ImagePromptStrength is the reference image influence (0-1).
	ImagePromptStrength float64 `json:"image_prompt_strength,omitempty" yaml:"image_prompt_strength,omitempty"`
}

// ImageResponse is the response from image generation.
type ImageResponse struct {
	Images []ImageData `json:"images"`
}

// ImageData contains image data.
type ImageData struct {
	// URL is the image URL.
	URL string `json:"url"`
}

// ================== Music Types ==================

// MusicRequest is the request for music generation.
type MusicRequest struct {
	// Model is the model name.
	Model string `json:"model,omitempty" yaml:"model,omitempty"`

	// Prompt is the music inspiration (10-300 characters).
	Prompt string `json:"prompt" yaml:"prompt"`

	// Lyrics is the song lyrics (10-600 characters).
	// Use \n to separate lines, supports tags: [Intro], [Verse], [Chorus], [Bridge], [Outro].
	Lyrics string `json:"lyrics" yaml:"lyrics"`

	// SampleRate is the sample rate: 16000, 24000, 32000, 44100.
	SampleRate int `json:"sample_rate,omitempty" yaml:"sample_rate,omitempty"`

	// Bitrate is the bitrate: 32000, 64000, 128000, 256000.
	Bitrate int `json:"bitrate,omitempty" yaml:"bitrate,omitempty"`

	// Format is the audio format: mp3, wav, pcm.
	Format string `json:"format,omitempty" yaml:"format,omitempty"`
}

// MusicResponse is the response from music generation.
type MusicResponse struct {
	// Audio is the decoded audio data.
	Audio []byte `json:"-"`

	// Duration is the audio duration in milliseconds.
	Duration int `json:"duration"`

	// ExtraInfo contains audio metadata.
	ExtraInfo *AudioInfo `json:"extra_info"`
}

// ================== File Types ==================

// FileInfo contains information about a file.
type FileInfo struct {
	// FileID is the file identifier (can be string or number from API).
	FileID FlexibleID `json:"file_id"`

	// Filename is the file name.
	Filename string `json:"filename"`

	// Bytes is the file size in bytes.
	Bytes int64 `json:"bytes"`

	// CreatedAt is the creation timestamp.
	CreatedAt int64 `json:"created_at"`

	// Purpose is the file purpose.
	Purpose string `json:"purpose"`

	// Status is the file status.
	Status string `json:"status,omitempty"`
}

// FileListResponse is the response from listing files.
type FileListResponse struct {
	// Files is the list of files.
	Files []FileInfo `json:"files"`
}
