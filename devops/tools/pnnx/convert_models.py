#!/usr/bin/env python3
"""Convert all speaker/VAD/denoise ONNX models to ncnn format.

This script handles models that require manual LSTM decomposition
(Silero VAD, DTLN) because ncnn's PNNX converter cannot handle
If control flow, scalar tensors, or torch.var.

Usage:
    python3 convert_models.py --pnnx /path/to/pnnx --output /path/to/output \\
        --eres2net model.onnx --silero-vad silero.jit \\
        --dtln1 dtln1.onnx --dtln2 dtln2.onnx

Output files:
    speaker_eres2net.ncnn.{param,bin}
    vad_silero.ncnn.{param,bin}
    denoise_dtln1.ncnn.{param,bin}
    denoise_dtln2.ncnn.{param,bin}
"""

import argparse
import os
import shutil
import subprocess
import sys
import tempfile

import numpy as np
import onnx
import torch
import torch.nn as nn
from onnx import numpy_helper


# ============================================================================
# Shared: Manual LSTM Cell (avoids chunk/var ops that ncnn can't handle)
# ============================================================================

class ManualLSTMCell(nn.Module):
    """LSTM cell with gates decomposed into separate Linear layers."""

    def __init__(self, input_size, hidden_size):
        super().__init__()
        self.hs = hidden_size
        self.i_x = nn.Linear(input_size, hidden_size)
        self.f_x = nn.Linear(input_size, hidden_size)
        self.g_x = nn.Linear(input_size, hidden_size)
        self.o_x = nn.Linear(input_size, hidden_size)
        self.i_h = nn.Linear(hidden_size, hidden_size)
        self.f_h = nn.Linear(hidden_size, hidden_size)
        self.g_h = nn.Linear(hidden_size, hidden_size)
        self.o_h = nn.Linear(hidden_size, hidden_size)

    def forward(self, x, h, c):
        i = torch.sigmoid(self.i_x(x) + self.i_h(h))
        f = torch.sigmoid(self.f_x(x) + self.f_h(h))
        g = torch.tanh(self.g_x(x) + self.g_h(h))
        o = torch.sigmoid(self.o_x(x) + self.o_h(h))
        c_new = f * c + i * g
        h_new = o * torch.tanh(c_new)
        return h_new, c_new

    def load_onnx(self, W, R, B):
        """Load from ONNX LSTM format. Gate order: i, o, f, c."""
        hs = self.hs
        Wb, Rb = B[:4 * hs], B[4 * hs:]
        with torch.no_grad():
            for gate, idx in [(self.i_x, 0), (self.o_x, 1), (self.f_x, 2), (self.g_x, 3)]:
                gate.weight.copy_(W[idx * hs:(idx + 1) * hs])
                gate.bias.copy_(Wb[idx * hs:(idx + 1) * hs])
            for gate, idx in [(self.i_h, 0), (self.o_h, 1), (self.f_h, 2), (self.g_h, 3)]:
                gate.weight.copy_(R[idx * hs:(idx + 1) * hs])
                gate.bias.copy_(Rb[idx * hs:(idx + 1) * hs])

    def load_pytorch(self, w_ih, w_hh, b_ih, b_hh):
        """Load from PyTorch LSTM format. Gate order: i, f, g, o."""
        hs = self.hs
        with torch.no_grad():
            for gate, idx in [(self.i_x, 0), (self.f_x, 1), (self.g_x, 2), (self.o_x, 3)]:
                gate.weight.copy_(w_ih[idx * hs:(idx + 1) * hs])
                gate.bias.copy_(b_ih[idx * hs:(idx + 1) * hs])
            for gate, idx in [(self.i_h, 0), (self.f_h, 1), (self.g_h, 2), (self.o_h, 3)]:
                gate.weight.copy_(w_hh[idx * hs:(idx + 1) * hs])
                gate.bias.copy_(b_hh[idx * hs:(idx + 1) * hs])


def onnx_weights(path):
    m = onnx.load(path)
    return {i.name: torch.tensor(numpy_helper.to_array(i).copy()) for i in m.graph.initializer}


def pnnx_export(model, name, shapes, inputs, pnnx_bin, output_dir):
    """Trace a PyTorch model and convert to ncnn via PNNX."""
    pt_path = os.path.join(output_dir, f"{name}.pt")
    traced = torch.jit.trace(model, inputs)
    traced.save(pt_path)

    shape_str = ",".join(f"[{','.join(str(d) for d in s)}]f32" for s in shapes)
    subprocess.run([pnnx_bin, pt_path, f"inputshape={shape_str}"],
                   capture_output=True, cwd=output_dir)

    param = os.path.join(output_dir, f"{name}.ncnn.param")
    bin_ = os.path.join(output_dir, f"{name}.ncnn.bin")
    if not os.path.exists(param) or not os.path.exists(bin_):
        raise RuntimeError(f"PNNX conversion failed for {name}")

    return param, bin_


# ============================================================================
# ERes2Net (speaker embedding) — direct PNNX conversion
# ============================================================================

def convert_eres2net(onnx_path, pnnx_bin, output_dir):
    name = "speaker_eres2net"
    work = tempfile.mkdtemp()
    try:
        shutil.copy(onnx_path, os.path.join(work, "m.onnx"))
        subprocess.run([pnnx_bin, "m.onnx", 'inputshape=[1,40,80]'],
                       capture_output=True, cwd=work)
        for ext in [".ncnn.param", ".ncnn.bin"]:
            src = os.path.join(work, f"m{ext}")
            dst = os.path.join(output_dir, f"{name}{ext}")
            shutil.copy(src, dst)
        return True
    finally:
        shutil.rmtree(work, ignore_errors=True)


# ============================================================================
# Silero VAD 16k — manual LSTM decomposition
# ============================================================================

class SileroVAD16k(nn.Module):
    def __init__(self):
        super().__init__()
        self.stft_conv = nn.Conv1d(1, 258, 256, stride=128, bias=False)
        self.enc0 = nn.Conv1d(129, 128, 3, padding=1)
        self.enc1 = nn.Conv1d(128, 64, 3, padding=1)
        self.enc2 = nn.Conv1d(64, 64, 3, padding=1)
        self.enc3 = nn.Conv1d(64, 128, 3, padding=1)
        self.lstm = ManualLSTMCell(128, 128)
        self.output_proj = nn.Linear(128, 1)

    def forward(self, audio, h, c):
        x = nn.functional.pad(audio.unsqueeze(1), (64, 64), mode='reflect')
        x = self.stft_conv(x)
        real, imag = x[:, :129, :], x[:, 129:, :]
        x = torch.sqrt(real * real + imag * imag + 1e-7)
        x = torch.relu(self.enc0(x))
        x = torch.relu(self.enc1(x))
        x = torch.relu(self.enc2(x))
        x = torch.relu(self.enc3(x))
        x = x.mean(dim=2)
        h_new, c_new = self.lstm(x, h, c)
        prob = torch.sigmoid(self.output_proj(torch.relu(h_new)))
        return prob, h_new, c_new


def convert_silero_vad(jit_path, pnnx_bin, output_dir):
    orig = torch.jit.load(jit_path)
    sd = dict(orig.state_dict())

    model = SileroVAD16k()
    model.eval()
    with torch.no_grad():
        model.stft_conv.weight.copy_(sd['_model.stft.forward_basis_buffer'])
        for i, (enc, n) in enumerate([(model.enc0, '0'), (model.enc1, '1'),
                                       (model.enc2, '2'), (model.enc3, '3')]):
            enc.weight.copy_(sd[f'_model.encoder.{n}.reparam_conv.weight'])
            enc.bias.copy_(sd[f'_model.encoder.{n}.reparam_conv.bias'])
        model.lstm.load_pytorch(
            sd['_model.decoder.rnn.weight_ih'], sd['_model.decoder.rnn.weight_hh'],
            sd['_model.decoder.rnn.bias_ih'], sd['_model.decoder.rnn.bias_hh'])
        model.output_proj.weight.copy_(sd['_model.decoder.decoder.2.weight'].squeeze(2))
        model.output_proj.bias.copy_(sd['_model.decoder.decoder.2.bias'])

    audio = torch.randn(1, 512) * 0.1
    h0, c0 = torch.zeros(1, 128), torch.zeros(1, 128)
    pnnx_export(model, "vad_silero", [(1, 512), (1, 128), (1, 128)],
                (audio, h0, c0), pnnx_bin, output_dir)
    return True


# ============================================================================
# DTLN Model 1 (STFT magnitude → mask)
# ============================================================================

class DTLNModel1(nn.Module):
    def __init__(self):
        super().__init__()
        self.lstm1 = ManualLSTMCell(257, 128)
        self.lstm2 = ManualLSTMCell(128, 128)
        self.fc = nn.Linear(128, 257)

    def forward(self, mag, h1, c1, h2, c2):
        h1n, c1n = self.lstm1(mag, h1, c1)
        h2n, c2n = self.lstm2(h1n, h2, c2)
        return torch.sigmoid(self.fc(h2n)), h1n, c1n, h2n, c2n


def convert_dtln1(onnx_path, pnnx_bin, output_dir):
    w = onnx_weights(onnx_path)

    model = DTLNModel1()
    model.eval()
    with torch.no_grad():
        model.lstm1.load_onnx(w['lstm_4_W'][0], w['lstm_4_R'][0], w['lstm_4_B'][0])
        model.lstm2.load_onnx(w['lstm_5_W'][0], w['lstm_5_R'][0], w['lstm_5_B'][0])
        model.fc.weight.copy_(w['dense_2/kernel:0'].T)
        model.fc.bias.copy_(w['dense_2/bias:0'])

    z = torch.zeros(1, 128)
    mag = torch.randn(1, 257) * 0.1
    pnnx_export(model, "denoise_dtln1",
                [(1, 257)] + [(1, 128)] * 4,
                (mag, z, z, z, z), pnnx_bin, output_dir)
    return True


# ============================================================================
# DTLN Model 2 (time-domain enhancement)
# ============================================================================

class DTLNModel2(nn.Module):
    def __init__(self):
        super().__init__()
        self.enc = nn.Linear(512, 256, bias=False)
        self.ln_g = nn.Parameter(torch.ones(256))
        self.ln_b = nn.Parameter(torch.zeros(256))
        self.lstm1 = ManualLSTMCell(256, 128)
        self.lstm2 = ManualLSTMCell(128, 128)
        self.dense = nn.Linear(128, 256)
        self.dec = nn.Linear(256, 512, bias=False)

    def forward(self, x, h1, c1, h2, c2):
        enc_out = self.enc(x)
        m = enc_out.mean(-1, keepdim=True)
        diff = enc_out - m
        var = torch.mean(diff * diff, dim=-1, keepdim=True)
        ln_out = diff / torch.sqrt(var + 1e-7) * self.ln_g + self.ln_b
        h1n, c1n = self.lstm1(ln_out, h1, c1)
        h2n, c2n = self.lstm2(h1n, h2, c2)
        # Sigmoid mask applied to enc output (not raw dense output).
        mask = torch.sigmoid(self.dense(h2n))
        masked = enc_out * mask
        return self.dec(masked), h1n, c1n, h2n, c2n


def convert_dtln2(onnx_path, pnnx_bin, output_dir):
    # Note: DTLN2 weights come from an ONNX export of a TF/Keras model.
    # The ONNX export decomposes TF LSTM into MatMul ops with ONNX gate
    # ordering (i, o, f, c). We use load_onnx() here, not load_pytorch().
    # The .T transpose is needed because ONNX MatMul stores weights transposed.
    w = onnx_weights(onnx_path)

    model = DTLNModel2()
    model.eval()
    with torch.no_grad():
        model.enc.weight.copy_(w['conv1d_2/kernel:0'].squeeze(2))
        model.ln_g.copy_(w['model_2/instant_layer_normalization_1/mul/ReadVariableOp/resource:0'])
        model.ln_b.copy_(w['model_2/instant_layer_normalization_1/add_1/ReadVariableOp/resource:0'])
        # DTLN2 LSTM weights are from TF/Keras decomposed MatMul ops.
        # TF gate order: i, f, c, o — matches load_pytorch (i, f, g, o).
        # NOT load_onnx (i, o, f, c) which is for ONNX LSTM op format.
        model.lstm1.load_pytorch(
            w['model_2/lstm_6/MatMul/ReadVariableOp/resource:0'].T.contiguous(),
            w['model_2/lstm_6/MatMul_1/ReadVariableOp/resource:0'].T.contiguous(),
            w['model_2/lstm_6/BiasAdd/ReadVariableOp/resource:0'],
            torch.zeros(512))
        model.lstm2.load_pytorch(
            w['model_2/lstm_7/MatMul/ReadVariableOp/resource:0'].T.contiguous(),
            w['model_2/lstm_7/MatMul_1/ReadVariableOp/resource:0'].T.contiguous(),
            w['model_2/lstm_7/BiasAdd/ReadVariableOp/resource:0'],
            torch.zeros(512))
        model.dense.weight.copy_(w['model_2/dense_3/Tensordot/Reshape_1:0'].T)
        model.dense.bias.copy_(w['model_2/dense_3/BiasAdd/ReadVariableOp/resource:0'])
        model.dec.weight.copy_(w['conv1d_3/kernel:0'].squeeze(2))

    z = torch.zeros(1, 128)
    feat = torch.randn(1, 512) * 0.1
    pnnx_export(model, "denoise_dtln2",
                [(1, 512)] + [(1, 128)] * 4,
                (feat, z, z, z, z), pnnx_bin, output_dir)
    return True


# ============================================================================
# Main
# ============================================================================

def main():
    p = argparse.ArgumentParser(description=__doc__,
                                formatter_class=argparse.RawDescriptionHelpFormatter)
    p.add_argument("--pnnx", required=True, help="Path to pnnx binary")
    p.add_argument("--output", required=True, help="Output directory")
    p.add_argument("--eres2net", help="ERes2Net ONNX model path")
    p.add_argument("--silero-vad", help="Silero VAD JIT model path")
    p.add_argument("--dtln1", help="DTLN model 1 ONNX path")
    p.add_argument("--dtln2", help="DTLN model 2 ONNX path")
    args = p.parse_args()

    os.makedirs(args.output, exist_ok=True)

    if args.eres2net:
        print("Converting ERes2Net...")
        convert_eres2net(args.eres2net, args.pnnx, args.output)
        print("  OK")

    if args.silero_vad:
        print("Converting Silero VAD 16k...")
        convert_silero_vad(args.silero_vad, args.pnnx, args.output)
        print("  OK")

    if args.dtln1:
        print("Converting DTLN Model 1...")
        convert_dtln1(args.dtln1, args.pnnx, args.output)
        print("  OK")

    if args.dtln2:
        print("Converting DTLN Model 2...")
        convert_dtln2(args.dtln2, args.pnnx, args.output)
        print("  OK")

    # Summary
    print("\nGenerated files:")
    for f in sorted(os.listdir(args.output)):
        if f.endswith(".ncnn.param") or f.endswith(".ncnn.bin"):
            size = os.path.getsize(os.path.join(args.output, f))
            print(f"  {f}: {size / 1024:.0f} KB")


if __name__ == "__main__":
    main()
