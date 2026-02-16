/* C shim for ONNX Runtime C API.
 *
 * The ORT C API uses a function pointer table (OrtApi struct).
 * Calling through function pointers from Rust FFI is cumbersome,
 * so we provide thin C wrappers (same approach as Go's CGo helpers).
 */

#include <onnxruntime_c_api.h>
#include <stdlib.h>
#include <string.h>

/* Get the ORT API pointer. */
const OrtApi* ort_api(void) {
    return OrtGetApiBase()->GetApi(ORT_API_VERSION);
}

/* Create environment. */
OrtStatus* ort_create_env(const OrtApi* api, const char* name, OrtEnv** out) {
    return api->CreateEnv(ORT_LOGGING_LEVEL_WARNING, name, out);
}

/* Create session options. */
OrtStatus* ort_create_session_options(const OrtApi* api, OrtSessionOptions** out) {
    return api->CreateSessionOptions(out);
}

/* Create session from memory. */
OrtStatus* ort_create_session_from_memory(const OrtApi* api, OrtEnv* env,
    const void* model_data, size_t model_data_len, OrtSessionOptions* opts, OrtSession** out) {
    return api->CreateSessionFromArray(env, model_data, model_data_len, opts, out);
}

/* Create tensor with float data. */
OrtStatus* ort_create_tensor_float(const OrtApi* api, OrtMemoryInfo* info,
    float* data, size_t data_len, int64_t* shape, size_t shape_len, OrtValue** out) {
    return api->CreateTensorWithDataAsOrtValue(info, data, data_len * sizeof(float),
        shape, shape_len, ONNX_TENSOR_ELEMENT_DATA_TYPE_FLOAT, out);
}

/* Create CPU memory info. */
OrtStatus* ort_create_cpu_memory_info(const OrtApi* api, OrtMemoryInfo** out) {
    return api->CreateCpuMemoryInfo(OrtArenaAllocator, OrtMemTypeDefault, out);
}

/* Run session. */
OrtStatus* ort_run(const OrtApi* api, OrtSession* session,
    const char** input_names, const OrtValue* const* inputs, size_t num_inputs,
    const char** output_names, size_t num_outputs, OrtValue** outputs) {
    return api->Run(session, NULL, input_names, inputs, num_inputs,
        output_names, num_outputs, outputs);
}

/* Get tensor float data pointer. */
OrtStatus* ort_get_tensor_float_data(const OrtApi* api, OrtValue* value, float** out) {
    return api->GetTensorMutableData(value, (void**)out);
}

/* Get tensor shape dimension count. */
OrtStatus* ort_get_tensor_ndim(const OrtApi* api, OrtValue* value, size_t* ndim) {
    OrtTensorTypeAndShapeInfo* info;
    OrtStatus* status = api->GetTensorTypeAndShape(value, &info);
    if (status) return status;
    status = api->GetDimensionsCount(info, ndim);
    api->ReleaseTensorTypeAndShapeInfo(info);
    return status;
}

/* Get tensor shape. */
OrtStatus* ort_get_tensor_shape(const OrtApi* api, OrtValue* value,
    int64_t* shape, size_t shape_len) {
    OrtTensorTypeAndShapeInfo* info;
    OrtStatus* status = api->GetTensorTypeAndShape(value, &info);
    if (status) return status;
    status = api->GetDimensions(info, shape, shape_len);
    api->ReleaseTensorTypeAndShapeInfo(info);
    return status;
}

/* Get error message. */
const char* ort_error_message(const OrtApi* api, OrtStatus* status) {
    return api->GetErrorMessage(status);
}

/* Release helpers. */
void ort_release_status(const OrtApi* api, OrtStatus* status) { api->ReleaseStatus(status); }
void ort_release_env(const OrtApi* api, OrtEnv* env) { api->ReleaseEnv(env); }
void ort_release_session(const OrtApi* api, OrtSession* s) { api->ReleaseSession(s); }
void ort_release_session_options(const OrtApi* api, OrtSessionOptions* o) { api->ReleaseSessionOptions(o); }
void ort_release_memory_info(const OrtApi* api, OrtMemoryInfo* i) { api->ReleaseMemoryInfo(i); }
void ort_release_value(const OrtApi* api, OrtValue* v) { api->ReleaseValue(v); }
