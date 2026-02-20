package cortex

import "github.com/haivivi/giztoy/go/pkg/kv"

func registerBuiltinSchemas(r *SchemaRegistry) {
	// --- creds ---

	r.Register(&Schema{
		Kind:     "creds/openai",
		Required: []string{"name", "api_key"},
		Optional: []string{"base_url"},
		KeyFunc: func(f map[string]any) kv.Key {
			return kv.Key{"creds", "openai", f["name"].(string)}
		},
	})

	r.Register(&Schema{
		Kind:     "creds/genai",
		Required: []string{"name", "api_key"},
		KeyFunc: func(f map[string]any) kv.Key {
			return kv.Key{"creds", "genai", f["name"].(string)}
		},
	})

	r.Register(&Schema{
		Kind:     "creds/minimax",
		Required: []string{"name", "api_key"},
		Optional: []string{"base_url", "default_model", "default_voice", "max_retries"},
		KeyFunc: func(f map[string]any) kv.Key {
			return kv.Key{"creds", "minimax", f["name"].(string)}
		},
	})

	r.Register(&Schema{
		Kind:     "creds/doubaospeech",
		Required: []string{"name", "app_id", "token"},
		Optional: []string{"api_key", "app_key", "base_url", "console_ak", "console_sk"},
		KeyFunc: func(f map[string]any) kv.Key {
			return kv.Key{"creds", "doubaospeech", f["name"].(string)}
		},
	})

	r.Register(&Schema{
		Kind:     "creds/dashscope",
		Required: []string{"name", "api_key"},
		Optional: []string{"workspace", "base_url"},
		KeyFunc: func(f map[string]any) kv.Key {
			return kv.Key{"creds", "dashscope", f["name"].(string)}
		},
	})

	// --- genx ---

	r.Register(&Schema{
		Kind:     "genx/generator",
		Required: []string{"name", "cred", "model"},
		Optional: []string{"max_tokens", "temperature", "top_p"},
		KeyFunc: func(f map[string]any) kv.Key {
			return kv.Key{"genx", "generator", f["name"].(string)}
		},
		ValidateFn: chainValidators(validateCredFormat, validateTemperature, validateMaxTokens),
	})

	r.Register(&Schema{
		Kind:     "genx/tts",
		Required: []string{"name", "cred", "voice_id"},
		KeyFunc: func(f map[string]any) kv.Key {
			return kv.Key{"genx", "tts", f["name"].(string)}
		},
		ValidateFn: validateCredFormat,
	})

	r.Register(&Schema{
		Kind:     "genx/asr",
		Required: []string{"name", "cred"},
		Optional: []string{"format", "sample_rate", "language"},
		KeyFunc: func(f map[string]any) kv.Key {
			return kv.Key{"genx", "asr", f["name"].(string)}
		},
		ValidateFn: validateCredFormat,
	})

	r.Register(&Schema{
		Kind:     "genx/realtime",
		Required: []string{"name", "cred"},
		Optional: []string{"voice", "model", "sample_rate"},
		KeyFunc: func(f map[string]any) kv.Key {
			return kv.Key{"genx", "realtime", f["name"].(string)}
		},
		ValidateFn: validateCredFormat,
	})

	r.Register(&Schema{
		Kind:     "genx/segmentor",
		Required: []string{"name", "cred"},
		KeyFunc: func(f map[string]any) kv.Key {
			return kv.Key{"genx", "segmentor", f["name"].(string)}
		},
		ValidateFn: validateCredFormat,
	})

	r.Register(&Schema{
		Kind:     "genx/profiler",
		Required: []string{"name", "cred"},
		KeyFunc: func(f map[string]any) kv.Key {
			return kv.Key{"genx", "profiler", f["name"].(string)}
		},
		ValidateFn: validateCredFormat,
	})

	// --- ctx ---

	r.Register(&Schema{
		Kind:     "ctx",
		Required: []string{"name"},
		Optional: []string{"kv", "storage", "vecstore", "embed"},
		KeyFunc: func(f map[string]any) kv.Key {
			return kv.Key{"ctx", f["name"].(string)}
		},
	})
}
