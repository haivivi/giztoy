// Package onnx provides Go bindings for the ONNX Runtime C API.
//
// ONNX Runtime is a cross-platform inference engine for ONNX models.
// This package wraps the C API, providing Go-native types for
// Environment, Session, and Tensor.
//
// # Architecture
//
// The package exposes three core types:
//
//   - [Env] — global environment (one per process)
//   - [Session] — loads and holds a model (.onnx file)
//   - [Tensor] — N-dimensional tensor for input/output data
//
// Usage flow:
//
//	env, _ := onnx.NewEnv("myapp")
//	defer env.Close()
//
//	session, _ := env.NewSession(modelData)
//	defer session.Close()
//
//	input, _ := onnx.NewTensor([]int64{1, 80, 40}, data)
//	defer input.Close()
//
//	outputs, _ := session.Run([]string{"in0"}, []*onnx.Tensor{input}, []string{"out0"})
//	result := outputs[0].FloatData()
//
// # Dynamic Linking
//
// ONNX Runtime is dynamically linked (.dylib/.so) via CGo.
// The pre-built library is downloaded by Bazel from GitHub releases.
//
// # Thread Safety
//
// Env is safe for concurrent use. Session.Run is thread-safe
// (ONNX Runtime uses internal locking).
package onnx

/*
#include <onnxruntime_c_api.h>
#include <stdlib.h>
#include <string.h>

// Helper: get the ORT API pointer.
static const OrtApi* ort_api() {
    return OrtGetApiBase()->GetApi(ORT_API_VERSION);
}

// Helper: create environment.
static OrtStatus* ort_create_env(const OrtApi* api, const char* name, OrtEnv** out) {
    return api->CreateEnv(ORT_LOGGING_LEVEL_WARNING, name, out);
}

// Helper: create session options.
static OrtStatus* ort_create_session_options(const OrtApi* api, OrtSessionOptions** out) {
    return api->CreateSessionOptions(out);
}

// Helper: create session from memory.
static OrtStatus* ort_create_session_from_memory(const OrtApi* api, OrtEnv* env,
    const void* model_data, size_t model_data_len, OrtSessionOptions* opts, OrtSession** out) {
    return api->CreateSessionFromArray(env, model_data, model_data_len, opts, out);
}

// Helper: create tensor with float data.
static OrtStatus* ort_create_tensor_float(const OrtApi* api, OrtMemoryInfo* info,
    float* data, size_t data_len, int64_t* shape, size_t shape_len, OrtValue** out) {
    return api->CreateTensorWithDataAsOrtValue(info, data, data_len * sizeof(float),
        shape, shape_len, ONNX_TENSOR_ELEMENT_DATA_TYPE_FLOAT, out);
}

// Helper: create CPU memory info.
static OrtStatus* ort_create_cpu_memory_info(const OrtApi* api, OrtMemoryInfo** out) {
    return api->CreateCpuMemoryInfo(OrtArenaAllocator, OrtMemTypeDefault, out);
}

// Helper: run session.
static OrtStatus* ort_run(const OrtApi* api, OrtSession* session,
    const char** input_names, const OrtValue* const* inputs, size_t num_inputs,
    const char** output_names, size_t num_outputs, OrtValue** outputs) {
    return api->Run(session, NULL, input_names, inputs, num_inputs,
        output_names, num_outputs, outputs);
}

// Helper: get tensor float data.
static OrtStatus* ort_get_tensor_float_data(const OrtApi* api, OrtValue* value, float** out) {
    return api->GetTensorMutableData(value, (void**)out);
}

// Helper: get tensor shape info.
static OrtStatus* ort_get_tensor_shape(const OrtApi* api, OrtValue* value,
    int64_t* shape, size_t shape_len) {
    OrtTensorTypeAndShapeInfo* info;
    OrtStatus* status = api->GetTensorTypeAndShape(value, &info);
    if (status) return status;
    status = api->GetDimensions(info, shape, shape_len);
    api->ReleaseTensorTypeAndShapeInfo(info);
    return status;
}

// Helper: get tensor shape dimension count.
static OrtStatus* ort_get_tensor_ndim(const OrtApi* api, OrtValue* value, size_t* ndim) {
    OrtTensorTypeAndShapeInfo* info;
    OrtStatus* status = api->GetTensorTypeAndShape(value, &info);
    if (status) return status;
    status = api->GetDimensionsCount(info, ndim);
    api->ReleaseTensorTypeAndShapeInfo(info);
    return status;
}

// Helper: get error message.
static const char* ort_error_message(const OrtApi* api, OrtStatus* status) {
    return api->GetErrorMessage(status);
}

// Helper: release status.
static void ort_release_status(const OrtApi* api, OrtStatus* status) {
    api->ReleaseStatus(status);
}

// Release helpers.
static void ort_release_env(const OrtApi* api, OrtEnv* env) { api->ReleaseEnv(env); }
static void ort_release_session(const OrtApi* api, OrtSession* s) { api->ReleaseSession(s); }
static void ort_release_session_options(const OrtApi* api, OrtSessionOptions* o) { api->ReleaseSessionOptions(o); }
static void ort_release_memory_info(const OrtApi* api, OrtMemoryInfo* i) { api->ReleaseMemoryInfo(i); }
static void ort_release_value(const OrtApi* api, OrtValue* v) { api->ReleaseValue(v); }
*/
import "C"

import (
	"fmt"
	"runtime"
	"unsafe"
)

// api returns the global ORT API pointer.
func api() *C.OrtApi {
	return C.ort_api()
}

// checkStatus converts an OrtStatus to a Go error.
func checkStatus(status *C.OrtStatus) error {
	if status == nil {
		return nil
	}
	msg := C.GoString(C.ort_error_message(api(), status))
	C.ort_release_status(api(), status)
	return fmt.Errorf("onnx: %s", msg)
}

// --------------------------------------------------------------------------
// Env
// --------------------------------------------------------------------------

// Env is the ONNX Runtime environment. Create one per process.
type Env struct {
	env *C.OrtEnv
}

// NewEnv creates a new ONNX Runtime environment.
func NewEnv(name string) (*Env, error) {
	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))

	var env *C.OrtEnv
	if err := checkStatus(C.ort_create_env(api(), cName, &env)); err != nil {
		return nil, err
	}

	e := &Env{env: env}
	runtime.SetFinalizer(e, (*Env).Close)
	return e, nil
}

// NewSession creates a session from in-memory ONNX model data.
func (e *Env) NewSession(modelData []byte) (*Session, error) {
	if len(modelData) == 0 {
		return nil, fmt.Errorf("onnx: empty model data")
	}

	var opts *C.OrtSessionOptions
	if err := checkStatus(C.ort_create_session_options(api(), &opts)); err != nil {
		return nil, err
	}
	defer C.ort_release_session_options(api(), opts)

	var session *C.OrtSession
	if err := checkStatus(C.ort_create_session_from_memory(
		api(), e.env,
		unsafe.Pointer(&modelData[0]), C.size_t(len(modelData)),
		opts, &session,
	)); err != nil {
		return nil, err
	}

	s := &Session{session: session, pinned: modelData}
	runtime.SetFinalizer(s, (*Session).Close)
	return s, nil
}

// Close releases the environment.
func (e *Env) Close() error {
	if e.env != nil {
		C.ort_release_env(api(), e.env)
		e.env = nil
		runtime.SetFinalizer(e, nil)
	}
	return nil
}

// --------------------------------------------------------------------------
// Session
// --------------------------------------------------------------------------

// Session holds a loaded ONNX model.
type Session struct {
	session *C.OrtSession
	pinned  any // prevents GC of model data
}

// Run executes inference with the given inputs and output names.
// Returns output tensors. The caller must close each output tensor.
func (s *Session) Run(inputNames []string, inputs []*Tensor, outputNames []string) ([]*Tensor, error) {
	if len(inputNames) != len(inputs) {
		return nil, fmt.Errorf("onnx: input names/tensors length mismatch: %d vs %d", len(inputNames), len(inputs))
	}

	// Prepare C input names
	cInputNames := make([]*C.char, len(inputNames))
	for i, name := range inputNames {
		cInputNames[i] = C.CString(name)
		defer C.free(unsafe.Pointer(cInputNames[i]))
	}

	// Prepare C input values
	cInputs := make([]*C.OrtValue, len(inputs))
	for i, t := range inputs {
		cInputs[i] = t.value
	}

	// Prepare C output names
	cOutputNames := make([]*C.char, len(outputNames))
	for i, name := range outputNames {
		cOutputNames[i] = C.CString(name)
		defer C.free(unsafe.Pointer(cOutputNames[i]))
	}

	// Allocate output values
	cOutputs := make([]*C.OrtValue, len(outputNames))

	status := C.ort_run(api(), s.session,
		&cInputNames[0], &cInputs[0], C.size_t(len(inputs)),
		&cOutputNames[0], C.size_t(len(outputNames)), &cOutputs[0],
	)
	if err := checkStatus(status); err != nil {
		return nil, err
	}

	// Wrap outputs
	outputs := make([]*Tensor, len(outputNames))
	for i, val := range cOutputs {
		outputs[i] = &Tensor{value: val, owned: true}
		runtime.SetFinalizer(outputs[i], (*Tensor).Close)
	}
	return outputs, nil
}

// Close releases the session.
func (s *Session) Close() error {
	if s.session != nil {
		C.ort_release_session(api(), s.session)
		s.session = nil
		runtime.SetFinalizer(s, nil)
	}
	return nil
}

// --------------------------------------------------------------------------
// Tensor
// --------------------------------------------------------------------------

// Tensor is an N-dimensional tensor (OrtValue).
type Tensor struct {
	value  *C.OrtValue
	pinned any  // prevents GC of external data
	owned  bool // if true, Close releases the OrtValue
}

// NewTensor creates a float32 tensor with the given shape and data.
// The data slice must remain valid for the lifetime of the Tensor.
func NewTensor(shape []int64, data []float32) (*Tensor, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("onnx: empty tensor data")
	}

	// Validate shape vs data length
	total := int64(1)
	for _, d := range shape {
		total *= d
	}
	if int64(len(data)) < total {
		return nil, fmt.Errorf("onnx: tensor data too short: got %d, need %d", len(data), total)
	}

	var memInfo *C.OrtMemoryInfo
	if err := checkStatus(C.ort_create_cpu_memory_info(api(), &memInfo)); err != nil {
		return nil, err
	}
	defer C.ort_release_memory_info(api(), memInfo)

	var value *C.OrtValue
	if err := checkStatus(C.ort_create_tensor_float(
		api(), memInfo,
		(*C.float)(unsafe.Pointer(&data[0])),
		C.size_t(len(data)),
		(*C.int64_t)(unsafe.Pointer(&shape[0])),
		C.size_t(len(shape)),
		&value,
	)); err != nil {
		return nil, err
	}

	t := &Tensor{value: value, pinned: data, owned: true}
	runtime.SetFinalizer(t, (*Tensor).Close)
	return t, nil
}

// FloatData copies the tensor data into a new float32 slice.
func (t *Tensor) FloatData() ([]float32, error) {
	var ptr *C.float
	if err := checkStatus(C.ort_get_tensor_float_data(api(), t.value, &ptr)); err != nil {
		return nil, err
	}

	// Get shape to determine total elements
	var ndim C.size_t
	if err := checkStatus(C.ort_get_tensor_ndim(api(), t.value, &ndim)); err != nil {
		return nil, err
	}

	shape := make([]C.int64_t, int(ndim))
	if ndim > 0 {
		if err := checkStatus(C.ort_get_tensor_shape(api(), t.value, &shape[0], ndim)); err != nil {
			return nil, err
		}
	}

	total := 1
	for _, d := range shape {
		total *= int(d)
	}
	if total <= 0 {
		return nil, nil
	}

	out := make([]float32, total)
	C.memcpy(unsafe.Pointer(&out[0]), unsafe.Pointer(ptr), C.size_t(total*4))
	return out, nil
}

// Shape returns the tensor dimensions.
func (t *Tensor) Shape() ([]int64, error) {
	var ndim C.size_t
	if err := checkStatus(C.ort_get_tensor_ndim(api(), t.value, &ndim)); err != nil {
		return nil, err
	}

	if ndim == 0 {
		return nil, nil
	}

	shape := make([]int64, int(ndim))
	if err := checkStatus(C.ort_get_tensor_shape(api(), t.value, (*C.int64_t)(unsafe.Pointer(&shape[0])), ndim)); err != nil {
		return nil, err
	}
	return shape, nil
}

// Close releases the tensor.
func (t *Tensor) Close() error {
	if t.value != nil && t.owned {
		C.ort_release_value(api(), t.value)
		t.value = nil
		runtime.SetFinalizer(t, nil)
	}
	return nil
}
