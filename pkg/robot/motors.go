// Package robot provides abstractions for controlling robot arms.
package robot

// MotorName identifies a motor in the arm.
type MotorName string

// Motor names for the SO-101 arm.
const (
	ShoulderPan  MotorName = "shoulder_pan"
	ShoulderLift MotorName = "shoulder_lift"
	ElbowFlex    MotorName = "elbow_flex"
	WristFlex    MotorName = "wrist_flex"
	WristRoll    MotorName = "wrist_roll"
	Gripper      MotorName = "gripper"
)

// AllMotors returns all motor names in order (matching servo IDs 1-6).
func AllMotors() []MotorName {
	return []MotorName{
		ShoulderPan,
		ShoulderLift,
		ElbowFlex,
		WristFlex,
		WristRoll,
		Gripper,
	}
}
