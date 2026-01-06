package robot

import (
	"context"
	"fmt"

	"github.com/hipsterbrown/feetech-servo/feetech"
)

// Arm represents a robot arm with multiple servos.
type Arm struct {
	bus         *feetech.Bus
	group       *feetech.ServoGroup
	calibration Calibration
}

// NewArm creates and initializes an arm connection.
func NewArm(port string, cal Calibration) (*Arm, error) {
	// Open serial bus
	bus, err := feetech.NewBus(feetech.BusConfig{
		Port:     port,
		BaudRate: 1_000_000,
		Protocol: feetech.ProtocolSTS,
	})
	if err != nil {
		return nil, fmt.Errorf("open bus: %w", err)
	}

	// Create servo group from calibration IDs
	ids := cal.MotorIDs()
	group := feetech.NewServoGroupByIDs(bus, ids...)

	return &Arm{
		bus:         bus,
		group:       group,
		calibration: cal,
	}, nil
}

// Close closes the arm's bus connection.
func (a *Arm) Close() error {
	return a.bus.Close()
}

// Enable enables torque on all servos.
func (a *Arm) Enable(ctx context.Context) error {
	return a.group.EnableAll(ctx)
}

// Disable disables torque on all servos.
func (a *Arm) Disable(ctx context.Context) error {
	return a.group.DisableAll(ctx)
}

// ReadPositions reads current positions from all motors.
// Returns normalized positions in the range [-100, 100].
func (a *Arm) ReadPositions(ctx context.Context) (map[MotorName]float64, error) {
	// Read raw positions using sync read
	rawPositions, err := a.group.Positions(ctx)
	if err != nil {
		return nil, fmt.Errorf("read positions: %w", err)
	}

	// Normalize each position
	positions := make(map[MotorName]float64, len(rawPositions))
	for id, raw := range rawPositions {
		name, cal, ok := a.calibration.ByID(id)
		if !ok {
			continue
		}
		positions[name] = cal.Normalize(raw)
	}

	return positions, nil
}

// WritePositions writes target positions to all motors.
// Takes normalized positions in the range [-100, 100].
func (a *Arm) WritePositions(ctx context.Context, positions map[MotorName]float64) error {
	// Denormalize positions
	rawPositions := make(feetech.PositionMap, len(positions))
	for name, norm := range positions {
		cal, ok := a.calibration[name]
		if !ok {
			continue
		}
		rawPositions[cal.ID] = cal.Denormalize(norm)
	}

	// Write using sync write
	if err := a.group.SetPositions(ctx, rawPositions); err != nil {
		return fmt.Errorf("write positions: %w", err)
	}

	return nil
}
