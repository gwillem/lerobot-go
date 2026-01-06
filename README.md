# lerobot-go

A Go implementation for teleoperating robot arms, compatible with [HuggingFace LeRobot](https://github.com/huggingface/lerobot). Control your SO-101 robot arm with live visualization.

![lerobot-go teleoperation](https://buq.eu/screenshots/2fc3bd789c7e80936bad65d3.png)

## Features

- **Teleoperation** - Control follower arm by moving leader arm at 60Hz
- **Live Position Graphs** - Terminal UI with streaming multi-line charts showing all 6 servo positions
- **Interactive Setup** - Scan, identify, and calibrate robot arms with guided workflow
- **Mirroring** - Optional mirror mode if your robot arms are positioned opposite each other

## Supported Hardware

- **SO-101 Robot Arm** (SO-ARM100) - 6-DOF robotic manipulator
- **Feetech STS3215 Servos** - Smart serial bus servos with position feedback
- **USB Serial Adapters** - Standard USB-to-TTL serial communication

## Installation

```bash
go install github.com/gwillem/lerobot/cmd/lerobot@latest
```

Or build from source:

```bash
git clone https://github.com/gwillem/lerobot.git
cd lerobot
go build ./cmd/lerobot
```

## Quick Start

### 1. Setup Robot Arms

Run the setup wizard to detect, identify, and calibrate your SO-101 arms:

```bash
lerobot setup
```

This will:

- Scan all serial ports for Feetech servos
- Wiggle each arm for identification (select leader/follower)
- Guide you through calibration (move joints to record min/max range)
- Save configuration to `lerobot.json`

### 2. Start Teleoperation

```bash
lerobot teleoperate
```

The terminal UI displays:

- Real-time position graph with 6 colored lines (one per motor)
- Color-coded legend for each joint
- Live log messages

Press `q` or `Ctrl+C` to stop.

## Command Line Options

### teleoperate

| Flag       | Default | Description                                               |
| ---------- | ------- | --------------------------------------------------------- |
| `--hz`     | `60`    | Control loop frequency in Hz                              |
| `--mirror` | `false` | Mirror mode: invert shoulder_pan and wrist_roll positions |

Example:

```bash
lerobot teleoperate --hz 30 --mirror
```

## Configuration

Configuration is stored in `lerobot.json`:

```json
{
  "leader": {
    "port": "/dev/cu.usbmodem1234",
    "calibration": {
      "shoulder_pan": { "id": 1, "range_min": 823, "range_max": 3540 },
      "shoulder_lift": { "id": 2, "range_min": 1000, "range_max": 3000 },
      ...
    }
  },
  "follower": {
    "port": "/dev/cu.usbmodem5678",
    "calibration": { ... }
  }
}
```

Run `lerobot setup` to regenerate this file.

## Architecture

```
lerobot-go/
├── cmd/
│   └── lerobot/           # CLI with setup and teleoperate commands
├── pkg/
│   ├── robot/             # Arm control, calibration, and config
│   └── teleop/            # Teleoperation controller
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

Thanks to the awesome [feetech-servo](https://github.com/hipsterbrown/feetech-servo) package for the Feetech servo interface.

## Related Projects

- [HuggingFace LeRobot](https://github.com/huggingface/lerobot) - Python robotics framework for imitation learning
- [SO-ARM100](https://github.com/TheRobotStudio/SO-ARM100) - Open-source robot arm design
- [Feetech STS3215](https://www.feetechrc.com/) - Smart serial servo manufacturer

## License

MIT License
