#!/usr/bin/env python3
"""Convert ONNX/JIT models to ncnn format.

This script handles models that require manual LSTM decomposition
(Silero VAD) because ncnn's PNNX converter cannot handle
If control flow, scalar tensors, or torch.var.

Usage:
    python3 convert_models.py --pnnx /path/to/pnnx --output /path/to/output \\
        --silero-vad silero.jit

Output files:
    vad_silero.ncnn.{param,bin}
"""

import argparse
import os
import shutil
import subprocess
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
# Main
# ============================================================================

def main():
    p = argparse.ArgumentParser(description=__doc__,
                                formatter_class=argparse.RawDescriptionHelpFormatter)
    p.add_argument("--pnnx", required=True, help="Path to pnnx binary")
    p.add_argument("--output", required=True, help="Output directory")
    p.add_argument("--silero-vad", help="Silero VAD JIT model path")
    args = p.parse_args()

    os.makedirs(args.output, exist_ok=True)

    if args.silero_vad:
        print("Converting Silero VAD 16k...")
        convert_silero_vad(args.silero_vad, args.pnnx, args.output)
        print("  OK")

    # Summary
    print("\nGenerated files:")
    for f in sorted(os.listdir(args.output)):
        if f.endswith(".ncnn.param") or f.endswith(".ncnn.bin"):
            size = os.path.getsize(os.path.join(args.output, f))
            print(f"  {f}: {size / 1024:.0f} KB")


if __name__ == "__main__":
    main()
