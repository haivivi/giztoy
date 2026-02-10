# Model Conversion (ONNX → ncnn)

## Pre-built models

Pre-converted ncnn models are checked in at `data/models/ncnn/` via git lfs.
You normally don't need to run conversion.

## Regenerating models

If you need to re-convert from ONNX sources:

```bash
# Prerequisites
pip3 install numpy onnx torch

# Build (manual targets, not included in bazel build //...)
bazel build //devops/tools/pnnx:models

# Copy to data/
cp bazel-bin/devops/tools/pnnx/*.ncnn.{param,bin} data/models/ncnn/
```

## TODO: ONNX golden comparison tests

The ncnn models should produce output identical (within tolerance) to the
original ONNX models. To verify pnnx conversion correctness:

1. Generate golden input/output pairs by running ONNX models in Python
2. Save as JSON/binary test fixtures in `testdata/models/`
3. Go tests load the same input, run ncnn inference, compare output

This would catch pnnx conversion issues (e.g., gate order, weight mapping).

See: https://github.com/haivivi/giztoy/pull/75 for discussion.
