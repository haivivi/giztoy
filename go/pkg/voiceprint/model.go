package voiceprint

// Model extracts speaker embedding vectors from raw audio.
//
// The input audio must be PCM16 signed little-endian, 16kHz, mono.
// The output is a dense float32 vector whose dimensionality is
// returned by Dimension().
//
// Typical implementations use ONNX Runtime to run a speaker
// verification model (e.g., ECAPA-TDNN, ResNet) that produces
// a 192-dimensional embedding per audio segment.
//
// # Audio Requirements
//
//   - Format: PCM16 signed little-endian
//   - Sample rate: 16000 Hz
//   - Channels: 1 (mono)
//   - Minimum duration: ~400ms (6400 samples) for meaningful embeddings
//
// # Thread Safety
//
// Implementations must be safe for concurrent use. Multiple goroutines
// may call Extract simultaneously.
type Model interface {
	// Extract computes a speaker embedding from raw PCM16 audio.
	// The audio slice must contain PCM16 signed little-endian samples
	// at 16kHz mono. Returns a float32 vector of length Dimension().
	Extract(audio []byte) ([]float32, error)

	// Dimension returns the dimensionality of the embedding vectors
	// produced by Extract (e.g., 192).
	Dimension() int

	// Close releases any resources held by the model (e.g., ONNX session).
	Close() error
}
