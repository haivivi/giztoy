// Package ncnn provides Go bindings for the ncnn neural network inference
// framework via CGo static linking.
//
// ncnn is a high-performance inference framework optimized for mobile and
// embedded platforms. This package wraps the ncnn C API, providing Go-native
// types for Net (model), Extractor (inference session), and Mat (tensor).
//
// # Architecture
//
// The package exposes three core types:
//
//   - [Net] — loads and holds a model (.param graph + .bin weights)
//   - [Extractor] — runs inference on a loaded Net
//   - [Mat] — N-dimensional tensor for input/output data
//
// Usage flow:
//
//	net, _ := ncnn.NewNetFromMemory(paramData, binData)
//	defer net.Close()
//
//	ex, _ := net.NewExtractor()
//	defer ex.Close()
//
//	ex.SetInput("in0", inputMat)
//	output, _ := ex.Extract("out0")
//	data := output.FloatData()
//
// # Static Linking
//
// ncnn is statically linked (.a) into the Go binary via CGo.
// No external shared libraries are needed at runtime.
//
// # Thread Safety
//
// Net is safe for concurrent use — multiple Extractors can run in parallel
// on the same Net. Each Extractor must be used from a single goroutine.
package ncnn

/*
#include <ncnn/c_api.h>
#include <stdlib.h>
#include <string.h>
*/
import "C"

import (
	"fmt"
	"runtime"
	"unsafe"
)

// Version returns the ncnn library version string.
func Version() string {
	return C.GoString(C.ncnn_version())
}

// --------------------------------------------------------------------------
// Net
// --------------------------------------------------------------------------

// Net holds a loaded ncnn model. Create with [NewNet] or [NewNetFromMemory].
// A Net is safe for concurrent use by multiple Extractors.
type Net struct {
	net C.ncnn_net_t
}

// NewNet loads a model from .param and .bin files on disk.
func NewNet(paramPath, binPath string) (*Net, error) {
	n := &Net{net: C.ncnn_net_create()}
	if n.net == nil {
		return nil, fmt.Errorf("ncnn: net_create failed")
	}

	cParam := C.CString(paramPath)
	defer C.free(unsafe.Pointer(cParam))
	if ret := C.ncnn_net_load_param(n.net, cParam); ret != 0 {
		C.ncnn_net_destroy(n.net)
		return nil, fmt.Errorf("ncnn: load_param %q: %d", paramPath, ret)
	}

	cBin := C.CString(binPath)
	defer C.free(unsafe.Pointer(cBin))
	if ret := C.ncnn_net_load_model(n.net, cBin); ret != 0 {
		C.ncnn_net_destroy(n.net)
		return nil, fmt.Errorf("ncnn: load_model %q: %d", binPath, ret)
	}

	runtime.SetFinalizer(n, (*Net).Close)
	return n, nil
}

// NewNetFromMemory loads a model from in-memory .param and .bin data.
// This is the preferred constructor when the model is embedded in the
// binary via go:embed.
//
// paramData is the text content of the .param file.
// binData is the raw bytes of the .bin file.
// opts is an optional Option to configure the net (FP16, threads, etc.).
// The option must be set before loading for it to take effect.
func NewNetFromMemory(paramData, binData []byte, opts ...*Option) (*Net, error) {
	if len(paramData) == 0 {
		return nil, fmt.Errorf("ncnn: empty param data")
	}
	if len(binData) == 0 {
		return nil, fmt.Errorf("ncnn: empty bin data")
	}

	n := &Net{net: C.ncnn_net_create()}
	if n.net == nil {
		return nil, fmt.Errorf("ncnn: net_create failed")
	}

	// Apply option BEFORE loading (ncnn applies options during load).
	for _, opt := range opts {
		if opt != nil {
			C.ncnn_net_set_option(n.net, opt.opt)
		}
	}

	// ncnn_net_load_param_memory expects a null-terminated C string.
	cParam := C.CString(string(paramData))
	defer C.free(unsafe.Pointer(cParam))
	if ret := C.ncnn_net_load_param_memory(n.net, cParam); ret != 0 {
		C.ncnn_net_destroy(n.net)
		return nil, fmt.Errorf("ncnn: load_param_memory: %d", ret)
	}

	// ncnn_net_load_model_memory returns bytes consumed (>0) on success, <0 on error.
	if ret := C.ncnn_net_load_model_memory(n.net, (*C.uchar)(unsafe.Pointer(&binData[0]))); ret < 0 {
		C.ncnn_net_destroy(n.net)
		return nil, fmt.Errorf("ncnn: load_model_memory: %d", ret)
	}

	runtime.SetFinalizer(n, (*Net).Close)
	return n, nil
}

// SetOption applies a configured Option to this Net.
// Must be called before creating Extractors.
func (n *Net) SetOption(opt *Option) {
	C.ncnn_net_set_option(n.net, opt.opt)
}

// Option configures inference behavior for a Net.
type Option struct {
	opt C.ncnn_option_t
}

// NewOption creates a new Option with default settings.
// Returns nil if allocation fails.
func NewOption() *Option {
	opt := C.ncnn_option_create()
	if opt == nil {
		return nil
	}
	o := &Option{opt: opt}
	runtime.SetFinalizer(o, (*Option).Close)
	return o
}

// SetFP16 enables or disables FP16 optimizations.
// Disable for models with intermediate values >65504 (e.g., Silero VAD).
func (o *Option) SetFP16(enabled bool) *Option {
	v := C.int(0)
	if enabled {
		v = 1
	}
	C.ncnn_option_set_use_fp16_packed(o.opt, v)
	C.ncnn_option_set_use_fp16_storage(o.opt, v)
	C.ncnn_option_set_use_fp16_arithmetic(o.opt, v)
	return o
}

// SetNumThreads sets the number of CPU threads for inference.
func (o *Option) SetNumThreads(n int) *Option {
	C.ncnn_option_set_num_threads(o.opt, C.int(n))
	return o
}

// Close releases the option resources.
func (o *Option) Close() error {
	if o.opt != nil {
		C.ncnn_option_destroy(o.opt)
		o.opt = nil
		runtime.SetFinalizer(o, nil)
	}
	return nil
}

// NewExtractor creates a new inference session for this Net.
// The Extractor must be closed after use.
// Returns an error if the extractor cannot be created.
func (n *Net) NewExtractor() (*Extractor, error) {
	ex := C.ncnn_extractor_create(n.net)
	if ex == nil {
		return nil, fmt.Errorf("ncnn: extractor_create failed")
	}
	e := &Extractor{ex: ex}
	runtime.SetFinalizer(e, (*Extractor).Close)
	return e, nil
}

// Close releases the ncnn network resources.
func (n *Net) Close() error {
	if n.net != nil {
		C.ncnn_net_destroy(n.net)
		n.net = nil
		runtime.SetFinalizer(n, nil)
	}
	return nil
}

// --------------------------------------------------------------------------
// Extractor
// --------------------------------------------------------------------------

// Extractor runs inference on a loaded Net. Create with [Net.NewExtractor].
// An Extractor must be used from a single goroutine.
type Extractor struct {
	ex C.ncnn_extractor_t
}

// SetInput feeds a Mat as input to the named blob.
func (e *Extractor) SetInput(name string, mat *Mat) error {
	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))
	if ret := C.ncnn_extractor_input(e.ex, cName, mat.mat); ret != 0 {
		return fmt.Errorf("ncnn: extractor_input %q: %d", name, ret)
	}
	return nil
}

// Extract runs inference and returns the output Mat for the named blob.
// The caller must close the returned Mat.
func (e *Extractor) Extract(name string) (*Mat, error) {
	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))

	var m C.ncnn_mat_t
	if ret := C.ncnn_extractor_extract(e.ex, cName, &m); ret != 0 {
		return nil, fmt.Errorf("ncnn: extractor_extract %q: %d", name, ret)
	}

	mat := &Mat{mat: m}
	runtime.SetFinalizer(mat, (*Mat).Close)
	return mat, nil
}

// SetOption applies a configured Option to this extractor.
func (e *Extractor) SetOption(opt *Option) {
	C.ncnn_extractor_set_option(e.ex, opt.opt)
}

// Close releases the extractor resources.
func (e *Extractor) Close() error {
	if e.ex != nil {
		C.ncnn_extractor_destroy(e.ex)
		e.ex = nil
		runtime.SetFinalizer(e, nil)
	}
	return nil
}

// --------------------------------------------------------------------------
// Mat
// --------------------------------------------------------------------------

// Mat is an N-dimensional tensor. Create with [NewMat2D], [NewMat3D],
// or [NewMatFromFloat32].
type Mat struct {
	mat C.ncnn_mat_t
}

// NewMat2D creates a 2D Mat (h rows × w cols) backed by external float32 data.
// The data slice must remain valid for the lifetime of the Mat.
func NewMat2D(w, h int, data []float32) (*Mat, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("ncnn: NewMat2D called with empty data")
	}
	if len(data) < w*h {
		return nil, fmt.Errorf("ncnn: NewMat2D data too short: got %d, need %d (w=%d, h=%d)", len(data), w*h, w, h)
	}
	mat := C.ncnn_mat_create_external_2d(
		C.int(w), C.int(h),
		unsafe.Pointer(&data[0]),
		nil,
	)
	if mat == nil {
		return nil, fmt.Errorf("ncnn: mat_create_external_2d failed")
	}
	m := &Mat{mat: mat}
	runtime.SetFinalizer(m, (*Mat).Close)
	return m, nil
}

// NewMat3D creates a 3D Mat (c channels × h rows × w cols) backed by external data.
func NewMat3D(w, h, c int, data []float32) (*Mat, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("ncnn: NewMat3D called with empty data")
	}
	if len(data) < w*h*c {
		return nil, fmt.Errorf("ncnn: NewMat3D data too short: got %d, need %d (w=%d, h=%d, c=%d)", len(data), w*h*c, w, h, c)
	}
	mat := C.ncnn_mat_create_external_3d(
		C.int(w), C.int(h), C.int(c),
		unsafe.Pointer(&data[0]),
		nil,
	)
	if mat == nil {
		return nil, fmt.Errorf("ncnn: mat_create_external_3d failed")
	}
	m := &Mat{mat: mat}
	runtime.SetFinalizer(m, (*Mat).Close)
	return m, nil
}

// W returns the width (first dimension) of the Mat.
func (m *Mat) W() int { return int(C.ncnn_mat_get_w(m.mat)) }

// H returns the height (second dimension) of the Mat.
func (m *Mat) H() int { return int(C.ncnn_mat_get_h(m.mat)) }

// C returns the number of channels (third dimension) of the Mat.
func (m *Mat) C() int { return int(C.ncnn_mat_get_c(m.mat)) }

// FloatData copies the Mat data into a new float32 slice.
// The returned slice length is W * H * C.
func (m *Mat) FloatData() []float32 {
	ptr := C.ncnn_mat_get_data(m.mat)
	if ptr == nil {
		return nil
	}
	n := m.W() * m.H() * m.C()
	if n <= 0 {
		n = m.W()
	}
	if n <= 0 {
		return nil
	}
	out := make([]float32, n)
	C.memcpy(unsafe.Pointer(&out[0]), ptr, C.size_t(n*4))
	return out
}

// Close releases the Mat resources.
func (m *Mat) Close() error {
	if m.mat != nil {
		C.ncnn_mat_destroy(m.mat)
		m.mat = nil
		runtime.SetFinalizer(m, nil)
	}
	return nil
}
