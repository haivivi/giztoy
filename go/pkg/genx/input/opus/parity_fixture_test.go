package opus

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"sort"
	"testing"
	"time"
)

type parityFixture struct {
	Cases []parityCase `json:"cases"`
}

type parityCase struct {
	Name            string        `json:"name"`
	MaxLossMs       int64         `json:"max_loss_ms"`
	Inputs          []parityInput `json:"inputs"`
	ExpectedOpusHex []string      `json:"expected_opus_hex"`
}

type parityInput struct {
	Kind    string `json:"kind"`
	StampMs int64  `json:"stamp_ms,omitempty"`
	OpusHex string `json:"opus_hex,omitempty"`
	RawHex  string `json:"raw_hex,omitempty"`
}

type parsedFrame struct {
	stamp EpochMillis
	seq   int
	frame OpusFrame
}

func TestParityFixture_GoReference(t *testing.T) {
	b, err := os.ReadFile("testdata/parity_cases.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	var fx parityFixture
	if err := json.Unmarshal(b, &fx); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}

	for _, c := range fx.Cases {
		t.Run(c.Name, func(t *testing.T) {
			rawInputs := make([][]byte, 0, len(c.Inputs))
			for _, in := range c.Inputs {
				switch in.Kind {
				case "stamped":
					opusBytes, err := hex.DecodeString(in.OpusHex)
					if err != nil {
						t.Fatalf("decode opus_hex %q: %v", in.OpusHex, err)
					}
					rawInputs = append(rawInputs, MakeStamped(OpusFrame(opusBytes), EpochMillis(in.StampMs)))
				case "raw":
					raw, err := hex.DecodeString(in.RawHex)
					if err != nil {
						t.Fatalf("decode raw_hex %q: %v", in.RawHex, err)
					}
					rawInputs = append(rawInputs, raw)
				default:
					t.Fatalf("unknown input kind: %s", in.Kind)
				}
			}

			got := runReferencePipeline(rawInputs, time.Duration(c.MaxLossMs)*time.Millisecond)
			want := decodeHexList(t, c.ExpectedOpusHex)
			if len(got) != len(want) {
				t.Fatalf("len mismatch: got=%d want=%d", len(got), len(want))
			}
			for i := range want {
				if hex.EncodeToString(got[i]) != hex.EncodeToString(want[i]) {
					t.Fatalf("output[%d] mismatch: got=%x want=%x", i, got[i], want[i])
				}
			}
		})
	}
}

func runReferencePipeline(raw [][]byte, maxLoss time.Duration) [][]byte {
	parsed := make([]parsedFrame, 0, len(raw))
	for i, b := range raw {
		f, ts, ok := ParseStamped(b)
		if !ok {
			continue
		}
		parsed = append(parsed, parsedFrame{stamp: ts, seq: i, frame: f.Clone()})
	}

	sort.SliceStable(parsed, func(i, j int) bool {
		if parsed[i].stamp == parsed[j].stamp {
			return parsed[i].seq < parsed[j].seq
		}
		return parsed[i].stamp < parsed[j].stamp
	})

	var out [][]byte
	var lastEnd *EpochMillis
	for _, p := range parsed {
		if lastEnd != nil {
			gap := p.stamp.Sub(*lastEnd)
			if gap > 0 && gap <= maxLoss {
				for gap >= 20*time.Millisecond {
					out = append(out, OpusSilence20ms.Clone())
					gap -= 20 * time.Millisecond
				}
			}
		}
		out = append(out, p.frame.Clone())
		next := p.stamp + FromDuration(p.frame.Duration())
		lastEnd = &next
	}

	return out
}

func decodeHexList(t *testing.T, xs []string) [][]byte {
	t.Helper()
	out := make([][]byte, 0, len(xs))
	for _, x := range xs {
		b, err := hex.DecodeString(x)
		if err != nil {
			t.Fatalf("decode expected hex %q: %v", x, err)
		}
		out = append(out, b)
	}
	return out
}
