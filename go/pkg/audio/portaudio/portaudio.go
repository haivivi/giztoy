// Package portaudio provides Go bindings for the PortAudio library.
//
// This package uses CGO to interface with the PortAudio C library,
// providing a simple API for audio input/output operations.
//
// For go build: requires portaudio installed via pkg-config (brew install portaudio)
// For bazel build: uses the bundled portaudio library
package portaudio

/*
#cgo pkg-config: portaudio-2.0

#include <portaudio.h>
#include <stdlib.h>
#include <string.h>

// Wrapper functions using void* to avoid CGO type issues with PaStream
static PaError pa_open_stream(void **stream,
                              const PaStreamParameters *inputParams,
                              const PaStreamParameters *outputParams,
                              double sampleRate,
                              unsigned long framesPerBuffer,
                              PaStreamFlags streamFlags) {
    return Pa_OpenStream((PaStream**)stream, inputParams, outputParams, sampleRate,
                         framesPerBuffer, streamFlags, NULL, NULL);
}

static PaError pa_start_stream(void *stream) {
    return Pa_StartStream((PaStream*)stream);
}

static PaError pa_stop_stream(void *stream) {
    return Pa_StopStream((PaStream*)stream);
}

static PaError pa_close_stream(void *stream) {
    return Pa_CloseStream((PaStream*)stream);
}

static PaError pa_read_stream(void *stream, void *buffer, unsigned long frames) {
    return Pa_ReadStream((PaStream*)stream, buffer, frames);
}

static PaError pa_write_stream(void *stream, const void *buffer, unsigned long frames) {
    return Pa_WriteStream((PaStream*)stream, buffer, frames);
}
*/
import "C"

import (
	"errors"
	"fmt"
	"sync"
	"unsafe"
)

var (
	initOnce sync.Once
	initErr  error
)

// paError converts a PortAudio error code to a Go error.
func paError(code C.PaError) error {
	if code == C.paNoError {
		return nil
	}
	return errors.New(C.GoString(C.Pa_GetErrorText(code)))
}

// Initialize initializes the PortAudio library.
// It is safe to call multiple times.
func Initialize() error {
	initOnce.Do(func() {
		initErr = paError(C.Pa_Initialize())
	})
	return initErr
}

// Terminate terminates the PortAudio library.
func Terminate() error {
	return paError(C.Pa_Terminate())
}

// DeviceInfo contains information about an audio device.
type DeviceInfo struct {
	Index                    int
	Name                     string
	MaxInputChannels         int
	MaxOutputChannels        int
	DefaultLowInputLatency   float64
	DefaultHighInputLatency  float64
	DefaultLowOutputLatency  float64
	DefaultHighOutputLatency float64
	DefaultSampleRate        float64
	IsDefaultInput           bool
	IsDefaultOutput          bool
}

// Devices returns a list of available audio devices.
func Devices() ([]DeviceInfo, error) {
	if err := Initialize(); err != nil {
		return nil, err
	}

	count := int(C.Pa_GetDeviceCount())
	if count < 0 {
		return nil, paError(C.PaError(count))
	}

	defaultInput := int(C.Pa_GetDefaultInputDevice())
	defaultOutput := int(C.Pa_GetDefaultOutputDevice())

	devices := make([]DeviceInfo, count)
	for i := 0; i < count; i++ {
		info := C.Pa_GetDeviceInfo(C.PaDeviceIndex(i))
		if info == nil {
			continue
		}
		devices[i] = DeviceInfo{
			Index:                    i,
			Name:                     C.GoString(info.name),
			MaxInputChannels:         int(info.maxInputChannels),
			MaxOutputChannels:        int(info.maxOutputChannels),
			DefaultLowInputLatency:   float64(info.defaultLowInputLatency),
			DefaultHighInputLatency:  float64(info.defaultHighInputLatency),
			DefaultLowOutputLatency:  float64(info.defaultLowOutputLatency),
			DefaultHighOutputLatency: float64(info.defaultHighOutputLatency),
			DefaultSampleRate:        float64(info.defaultSampleRate),
			IsDefaultInput:           i == defaultInput,
			IsDefaultOutput:          i == defaultOutput,
		}
	}
	return devices, nil
}

// DefaultInputDevice returns the default input device.
func DefaultInputDevice() (*DeviceInfo, error) {
	if err := Initialize(); err != nil {
		return nil, err
	}

	idx := C.Pa_GetDefaultInputDevice()
	if idx == C.paNoDevice {
		return nil, errors.New("no default input device")
	}

	info := C.Pa_GetDeviceInfo(idx)
	if info == nil {
		return nil, errors.New("failed to get device info")
	}

	return &DeviceInfo{
		Index:                   int(idx),
		Name:                    C.GoString(info.name),
		MaxInputChannels:        int(info.maxInputChannels),
		DefaultLowInputLatency:  float64(info.defaultLowInputLatency),
		DefaultHighInputLatency: float64(info.defaultHighInputLatency),
		DefaultSampleRate:       float64(info.defaultSampleRate),
		IsDefaultInput:          true,
	}, nil
}

// DefaultOutputDevice returns the default output device.
func DefaultOutputDevice() (*DeviceInfo, error) {
	if err := Initialize(); err != nil {
		return nil, err
	}

	idx := C.Pa_GetDefaultOutputDevice()
	if idx == C.paNoDevice {
		return nil, errors.New("no default output device")
	}

	info := C.Pa_GetDeviceInfo(idx)
	if info == nil {
		return nil, errors.New("failed to get device info")
	}

	return &DeviceInfo{
		Index:                    int(idx),
		Name:                     C.GoString(info.name),
		MaxOutputChannels:        int(info.maxOutputChannels),
		DefaultLowOutputLatency:  float64(info.defaultLowOutputLatency),
		DefaultHighOutputLatency: float64(info.defaultHighOutputLatency),
		DefaultSampleRate:        float64(info.defaultSampleRate),
		IsDefaultOutput:          true,
	}, nil
}

// PrintDevices prints all available devices to stdout.
func PrintDevices() error {
	devices, err := Devices()
	if err != nil {
		return err
	}
	for _, d := range devices {
		marker := ""
		if d.IsDefaultInput {
			marker += " [DEFAULT INPUT]"
		}
		if d.IsDefaultOutput {
			marker += " [DEFAULT OUTPUT]"
		}
		fmt.Printf("%d: %s%s\n", d.Index, d.Name, marker)
		fmt.Printf("   Input channels: %d, Output channels: %d\n", d.MaxInputChannels, d.MaxOutputChannels)
		fmt.Printf("   Default sample rate: %.0f Hz\n", d.DefaultSampleRate)
	}
	return nil
}

// Stream represents an audio stream.
type Stream struct {
	stream     unsafe.Pointer
	buffer     unsafe.Pointer
	bufferSize int
	closed     bool
	mu         sync.Mutex
}

// openStream opens a PortAudio stream with the given parameters.
func openStream(inputChannels, outputChannels int, sampleRate float64, framesPerBuffer int) (*Stream, error) {
	if err := Initialize(); err != nil {
		return nil, err
	}

	var inputParams, outputParams *C.PaStreamParameters

	if inputChannels > 0 {
		inputDevice := C.Pa_GetDefaultInputDevice()
		if inputDevice == C.paNoDevice {
			return nil, errors.New("no default input device")
		}
		inputInfo := C.Pa_GetDeviceInfo(inputDevice)
		inputParams = &C.PaStreamParameters{
			device:                    inputDevice,
			channelCount:              C.int(inputChannels),
			sampleFormat:              C.paInt16,
			suggestedLatency:          inputInfo.defaultLowInputLatency,
			hostApiSpecificStreamInfo: nil,
		}
	}

	if outputChannels > 0 {
		outputDevice := C.Pa_GetDefaultOutputDevice()
		if outputDevice == C.paNoDevice {
			return nil, errors.New("no default output device")
		}
		outputInfo := C.Pa_GetDeviceInfo(outputDevice)
		outputParams = &C.PaStreamParameters{
			device:                    outputDevice,
			channelCount:              C.int(outputChannels),
			sampleFormat:              C.paInt16,
			suggestedLatency:          outputInfo.defaultLowOutputLatency,
			hostApiSpecificStreamInfo: nil,
		}
	}

	var paStream unsafe.Pointer
	err := paError(C.pa_open_stream(
		&paStream,
		inputParams,
		outputParams,
		C.double(sampleRate),
		C.ulong(framesPerBuffer),
		C.paClipOff,
	))
	if err != nil {
		return nil, err
	}

	// Calculate buffer size (samples * channels * sizeof(int16))
	channels := inputChannels
	if outputChannels > channels {
		channels = outputChannels
	}
	bufferSize := framesPerBuffer * channels * 2 // int16 = 2 bytes

	return &Stream{
		stream:     paStream,
		buffer:     C.malloc(C.size_t(bufferSize)),
		bufferSize: bufferSize,
	}, nil
}

// Start starts the audio stream.
func (s *Stream) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return errors.New("stream closed")
	}
	return paError(C.pa_start_stream(s.stream))
}

// Stop stops the audio stream.
func (s *Stream) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil
	}
	return paError(C.pa_stop_stream(s.stream))
}

// Close closes the audio stream.
func (s *Stream) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil
	}
	s.closed = true

	C.pa_stop_stream(s.stream)
	err := paError(C.pa_close_stream(s.stream))
	C.free(s.buffer)
	return err
}

// Read reads audio samples from an input stream.
func (s *Stream) Read(framesPerBuffer int) ([]int16, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil, errors.New("stream closed")
	}

	err := paError(C.pa_read_stream(s.stream, s.buffer, C.ulong(framesPerBuffer)))
	if err != nil {
		return nil, err
	}

	// Copy from C buffer to Go slice
	samples := make([]int16, framesPerBuffer)
	C.memcpy(unsafe.Pointer(&samples[0]), s.buffer, C.size_t(framesPerBuffer*2))
	return samples, nil
}

// Write writes audio samples to an output stream.
func (s *Stream) Write(samples []int16) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return errors.New("stream closed")
	}

	// Copy to C buffer
	C.memcpy(s.buffer, unsafe.Pointer(&samples[0]), C.size_t(len(samples)*2))
	return paError(C.pa_write_stream(s.stream, s.buffer, C.ulong(len(samples))))
}
