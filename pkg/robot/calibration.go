package robot

import (
	"encoding/json"
	"fmt"
	"os"
)

// MotorCalibration holds calibration data for a single motor.
type MotorCalibration struct {
	ID           int `json:"id"`
	DriveMode    int `json:"drive_mode"`
	HomingOffset int `json:"homing_offset"`
	RangeMin     int `json:"range_min"`
	RangeMax     int `json:"range_max"`
}

// Calibration holds calibration data for all motors, keyed by motor name.
type Calibration map[MotorName]MotorCalibration

// LoadCalibration loads calibration data from a JSON file.
func LoadCalibration(path string) (Calibration, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read calibration file: %w", err)
	}

	// Parse into a map with string keys first
	var raw map[string]MotorCalibration
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse calibration JSON: %w", err)
	}

	// Convert to Calibration with MotorName keys
	cal := make(Calibration, len(raw))
	for name, mc := range raw {
		cal[MotorName(name)] = mc
	}

	return cal, nil
}

// Normalize converts a raw servo position to a normalized value in the range [-100, 100].
func (c MotorCalibration) Normalize(raw int) float64 {
	rangeSize := float64(c.RangeMax - c.RangeMin)
	if rangeSize == 0 {
		return 0
	}
	return (float64(raw-c.RangeMin)/rangeSize)*200 - 100
}

// Denormalize converts a normalized value [-100, 100] to a raw servo position.
func (c MotorCalibration) Denormalize(norm float64) int {
	rangeSize := float64(c.RangeMax - c.RangeMin)
	return int((norm+100)/200*rangeSize) + c.RangeMin
}

// MotorIDs returns the servo IDs for all motors in the calibration.
func (c Calibration) MotorIDs() []int {
	ids := make([]int, 0, len(c))
	// Use AllMotors() to ensure consistent ordering
	for _, name := range AllMotors() {
		if mc, ok := c[name]; ok {
			ids = append(ids, mc.ID)
		}
	}
	return ids
}

// ByID returns motor name and calibration for a given servo ID.
func (c Calibration) ByID(id int) (MotorName, MotorCalibration, bool) {
	for name, mc := range c {
		if mc.ID == id {
			return name, mc, true
		}
	}
	return "", MotorCalibration{}, false
}
