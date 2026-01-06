package robot

import (
	"math"
	"testing"
)

func TestMotorCalibration_Normalize(t *testing.T) {
	cal := MotorCalibration{
		RangeMin: 1000,
		RangeMax: 3000,
	}

	tests := []struct {
		raw      int
		expected float64
	}{
		{1000, -100.0}, // min -> -100
		{3000, 100.0},  // max -> 100
		{2000, 0.0},    // mid -> 0
		{1500, -50.0},  // quarter -> -50
		{2500, 50.0},   // three-quarter -> 50
	}

	for _, tt := range tests {
		got := cal.Normalize(tt.raw)
		if math.Abs(got-tt.expected) > 0.001 {
			t.Errorf("Normalize(%d) = %f, want %f", tt.raw, got, tt.expected)
		}
	}
}

func TestMotorCalibration_Denormalize(t *testing.T) {
	cal := MotorCalibration{
		RangeMin: 1000,
		RangeMax: 3000,
	}

	tests := []struct {
		norm     float64
		expected int
	}{
		{-100.0, 1000}, // -100 -> min
		{100.0, 3000},  // 100 -> max
		{0.0, 2000},    // 0 -> mid
		{-50.0, 1500},  // -50 -> quarter
		{50.0, 2500},   // 50 -> three-quarter
	}

	for _, tt := range tests {
		got := cal.Denormalize(tt.norm)
		if got != tt.expected {
			t.Errorf("Denormalize(%f) = %d, want %d", tt.norm, got, tt.expected)
		}
	}
}

func TestMotorCalibration_RoundTrip(t *testing.T) {
	cal := MotorCalibration{
		RangeMin: 823,
		RangeMax: 3540,
	}

	// Test round-trip: raw -> normalized -> raw
	for raw := cal.RangeMin; raw <= cal.RangeMax; raw += 100 {
		norm := cal.Normalize(raw)
		back := cal.Denormalize(norm)
		if math.Abs(float64(back-raw)) > 1 {
			t.Errorf("Round-trip failed: %d -> %f -> %d", raw, norm, back)
		}
	}
}

func TestCalibration_MotorIDs(t *testing.T) {
	cal := Calibration{
		ShoulderPan:  MotorCalibration{ID: 1},
		ShoulderLift: MotorCalibration{ID: 2},
		ElbowFlex:    MotorCalibration{ID: 3},
		WristFlex:    MotorCalibration{ID: 4},
		WristRoll:    MotorCalibration{ID: 5},
		Gripper:      MotorCalibration{ID: 6},
	}

	ids := cal.MotorIDs()
	expected := []int{1, 2, 3, 4, 5, 6}

	if len(ids) != len(expected) {
		t.Fatalf("MotorIDs returned %d IDs, want %d", len(ids), len(expected))
	}

	for i, id := range ids {
		if id != expected[i] {
			t.Errorf("MotorIDs()[%d] = %d, want %d", i, id, expected[i])
		}
	}
}

func TestCalibration_ByID(t *testing.T) {
	cal := Calibration{
		ShoulderPan: MotorCalibration{ID: 1, RangeMin: 100, RangeMax: 200},
		Gripper:     MotorCalibration{ID: 6, RangeMin: 300, RangeMax: 400},
	}

	// Test finding existing ID
	name, mc, ok := cal.ByID(1)
	if !ok {
		t.Fatal("ByID(1) returned false")
	}
	if name != ShoulderPan {
		t.Errorf("ByID(1) returned name %s, want shoulder_pan", name)
	}
	if mc.RangeMin != 100 {
		t.Errorf("ByID(1) returned wrong calibration: %+v", mc)
	}

	// Test non-existing ID
	_, _, ok = cal.ByID(99)
	if ok {
		t.Error("ByID(99) should return false")
	}
}
