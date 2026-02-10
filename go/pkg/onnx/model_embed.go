package onnx

import _ "embed"

// Embedded ONNX model files.
// Made available via Bazel embedsrcs in BUILD.bazel.

//go:embed speaker_eres2net.onnx
var speakerERes2NetData []byte

//go:embed denoise_nsnet2.onnx
var denoiseNSNet2Data []byte

func init() {
	RegisterModel(ModelSpeakerERes2Net, speakerERes2NetData)
	RegisterModel(ModelDenoiseNSNet2, denoiseNSNet2Data)
}
