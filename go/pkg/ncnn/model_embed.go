package ncnn

import _ "embed"

// Embedded ncnn model files.
// Made available via Bazel embedsrcs in BUILD.bazel.

//go:embed speaker_eres2net.ncnn.param
var speakerERes2NetParam []byte

//go:embed speaker_eres2net.ncnn.bin
var speakerERes2NetBin []byte

//go:embed vad_silero.ncnn.param
var vadSileroParam []byte

//go:embed vad_silero.ncnn.bin
var vadSileroBin []byte

func init() {
	RegisterModel(ModelSpeakerERes2Net, speakerERes2NetParam, speakerERes2NetBin)
	RegisterModel(ModelVADSilero, vadSileroParam, vadSileroBin)
}
