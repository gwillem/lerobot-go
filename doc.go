// Package lerobot provides teleoperation control for SO-101 robot arms.
//
// This is a Go implementation compatible with HuggingFace LeRobot, allowing
// you to control a follower arm by moving a leader arm in real-time.
//
// # Installation
//
//	go install github.com/gwillem/lerobot/cmd/lerobot@latest
//
// # Usage
//
// First, run setup to detect and calibrate your robot arms:
//
//	lerobot setup
//
// Then start teleoperation:
//
//	lerobot teleoperate
//
// # Packages
//
// The module is organized into the following packages:
//
//   - cmd/lerobot: CLI with setup and teleoperate commands
//   - pkg/robot: Arm control, calibration, and configuration
//   - pkg/teleop: Teleoperation controller
package lerobot
