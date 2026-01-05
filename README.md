# lerobot-go

A high-performance Go implementation for teleoperating robot arms, compatible with [HuggingFace LeRobot](https://github.com/huggingface/lerobot). Control your SO-101 robot arm with real-time visualization and seamless hardware integration.

## Features

- **Real-time Teleoperation** - Control follower arm by moving leader arm at 60Hz
- **Live Position Graphs** - Terminal UI with streaming multi-line charts showing all 6 servo positions
- **Automatic Port Detection** - Scan and identify robot arms with interactive servo wiggle test
- **LeRobot Calibration Compatible** - Reuse calibration files from HuggingFace LeRobot Python framework

## Supported Hardware

- **SO-101 Robot Arm** (SO-ARM100) - 6-DOF robotic manipulator
- **Feetech STS3215 Servos** - Smart serial bus servos with position feedback
- **USB Serial Adapters** - Standard USB-to-TTL serial communication

## Installation

```bash
go install github.com/yourusername/lerobot-go/cmd/lerobot-teleoperate@latest
go install github.com/yourusername/lerobot-go/cmd/robot-info@latest
```

Or build from source:

```bash
git clone https://github.com/yourusername/lerobot-go.git
cd lerobot-go
go build ./cmd/...
```

## Quick Start

### 1. Detect and Configure Robot Arms

Run the port scanner to automatically detect your SO-101 arms:

```bash
go run ./cmd/robot-info
```

This will:

- Scan all serial ports for Feetech servos
- Wiggle each arm for identification
- Prompt you to label leader and follower arms
- Save configuration to `lerobot.json`

### 2. Start Teleoperation

```bash
go run ./cmd/lerobot-teleoperate
```

The terminal UI displays:

- Real-time position graph with 6 colored lines (one per motor)
- Color-coded legend for each joint
- Live log messages

Press `q` or `Ctrl+C` to stop.

## Manual Configuration

If you prefer manual configuration, specify ports directly:

```bash
go run ./cmd/lerobot-teleoperate \
  --teleop.port /dev/tty.usbserial-1234 \
  --robot.port /dev/tty.usbserial-5678
```

### Command Line Options

| Flag            | Default     | Description                             |
| --------------- | ----------- | --------------------------------------- |
| `--robot.port`  | from config | Follower arm serial port                |
| `--robot.id`    | `follower`  | Robot identifier for calibration        |
| `--teleop.port` | from config | Leader arm serial port                  |
| `--teleop.id`   | `leader`    | Teleoperator identifier for calibration |
| `--hz`          | `60`        | Control loop frequency in Hz            |

## Calibration

### Using LeRobot Calibration Files

(todo: implement here)

Copy your existing LeRobot calibration files:

```bash
mkdir -p calibration
cp ~/.cache/huggingface/lerobot/calibration/robots/so101_follower/main_follower.json calibration/follower.json
cp ~/.cache/huggingface/lerobot/calibration/teleoperators/so101_leader/main_leader.json calibration/leader.json
```

### Calibration File Format

```json
{
  "shoulder_pan": {
    "id": 1,
    "drive_mode": 0,
    "homing_offset": 978,
    "range_min": 823,
    "range_max": 3540
  },
  "shoulder_lift": { ... },
  "elbow_flex": { ... },
  "wrist_flex": { ... },
  "wrist_roll": { ... },
  "gripper": { ... }
}
```

## Architecture

```
lerobot-go/
├── cmd/
│   ├── lerobot-teleoperate/  # Main teleoperation TUI
│   └── robot-info/           # Port scanner and configurator
├── pkg/
│   ├── robot/                # Arm control and calibration
│   └── teleop/               # Teleoperation controller
└── calibration/              # Calibration JSON files
```

### Motor Configuration

| Motor           | Servo ID | Description        |
| --------------- | -------- | ------------------ |
| `shoulder_pan`  | 1        | Base rotation      |
| `shoulder_lift` | 2        | Shoulder elevation |
| `elbow_flex`    | 3        | Elbow bend         |
| `wrist_flex`    | 4        | Wrist pitch        |
| `wrist_roll`    | 5        | Wrist rotation     |
| `gripper`       | 6        | Gripper open/close |

## Dependencies

- [feetech-servo](https://github.com/hipsterbrown/feetech-servo) - Feetech servo protocol

## Related Projects

- [HuggingFace LeRobot](https://github.com/huggingface/lerobot) - Python robotics framework for imitation learning
- [SO-ARM100](https://github.com/TheRobotStudio/SO-ARM100) - Open-source robot arm design
- [Feetech STS3215](https://www.feetechrc.com/) - Smart serial servo manufacturer

## License

MIT License
