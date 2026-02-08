package ncnn

import (
	"testing"
)

func TestVersion(t *testing.T) {
	v := Version()
	if v == "" {
		t.Error("Version() returned empty string")
	}
	t.Logf("ncnn version: %s", v)
}

func TestMat2D(t *testing.T) {
	data := make([]float32, 80*40) // 40 rows × 80 cols
	for i := range data {
		data[i] = float32(i) * 0.001
	}

	mat := NewMat2D(80, 40, data)
	defer mat.Close()

	if mat.W() != 80 {
		t.Errorf("W() = %d, want 80", mat.W())
	}
	if mat.H() != 40 {
		t.Errorf("H() = %d, want 40", mat.H())
	}
}

func TestMat3D(t *testing.T) {
	data := make([]float32, 10*20*3)
	mat := NewMat3D(10, 20, 3, data)
	defer mat.Close()

	if mat.W() != 10 {
		t.Errorf("W() = %d, want 10", mat.W())
	}
	if mat.H() != 20 {
		t.Errorf("H() = %d, want 20", mat.H())
	}
	if mat.C() != 3 {
		t.Errorf("C() = %d, want 3", mat.C())
	}
}

func TestMatFloatData(t *testing.T) {
	data := []float32{1.0, 2.0, 3.0, 4.0, 5.0}
	mat := NewMat2D(5, 1, data)
	defer mat.Close()

	out := mat.FloatData()
	if len(out) != 5 {
		t.Fatalf("FloatData len = %d, want 5", len(out))
	}
	for i, v := range out {
		if v != data[i] {
			t.Errorf("FloatData[%d] = %f, want %f", i, v, data[i])
		}
	}
}

func TestMatDoubleClose(t *testing.T) {
	data := []float32{1, 2, 3}
	mat := NewMat2D(3, 1, data)
	mat.Close()
	mat.Close() // should not panic
}

func TestNetClose(t *testing.T) {
	// Close without loading should not panic.
	n := &Net{}
	n.Close()
}
