// Package teleop provides teleoperation control for robot arms.
package teleop

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/gwillem/lerobot/pkg/robot"
)

// State represents the current state of teleoperation.
type State struct {
	Positions map[robot.MotorName]float64
	Timestamp time.Time
	Error     error
}

// Controller manages the teleoperation control loop.
type Controller struct {
	leader   *robot.Arm
	follower *robot.Arm
	hz       int
	mirror   bool

	mu       sync.RWMutex
	state    State
	running  bool
	stateCh  chan State
	logs     []string
	logCh    chan string
}

// Config holds configuration for the controller.
type Config struct {
	LeaderPort    string
	LeaderCalib   string
	FollowerPort  string
	FollowerCalib string
	Hz            int
	Mirror        bool // Invert positions for shoulder_pan (servo 1) and wrist_roll (servo 5)
}

// NewController creates a new teleoperation controller.
func NewController(cfg Config) (*Controller, error) {
	leader, err := robot.NewArm(robot.ArmConfig{
		Port:            cfg.LeaderPort,
		CalibrationPath: cfg.LeaderCalib,
	})
	if err != nil {
		return nil, fmt.Errorf("create leader arm: %w", err)
	}

	follower, err := robot.NewArm(robot.ArmConfig{
		Port:            cfg.FollowerPort,
		CalibrationPath: cfg.FollowerCalib,
	})
	if err != nil {
		leader.Close()
		return nil, fmt.Errorf("create follower arm: %w", err)
	}

	if cfg.Hz <= 0 {
		cfg.Hz = 60
	}

	return &Controller{
		leader:   leader,
		follower: follower,
		hz:       cfg.Hz,
		mirror:   cfg.Mirror,
		stateCh:  make(chan State, 1),
		logCh:    make(chan string, 10),
	}, nil
}

// Close closes the controller and releases resources.
func (c *Controller) Close() error {
	c.mu.Lock()
	c.running = false
	c.mu.Unlock()

	var errs []error
	if err := c.leader.Close(); err != nil {
		errs = append(errs, err)
	}
	if err := c.follower.Close(); err != nil {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return fmt.Errorf("close errors: %v", errs)
	}
	return nil
}

// States returns a channel that receives state updates.
func (c *Controller) States() <-chan State {
	return c.stateCh
}

// Logs returns a channel that receives log messages.
func (c *Controller) Logs() <-chan string {
	return c.logCh
}

// Hz returns the control frequency.
func (c *Controller) Hz() int {
	return c.hz
}

func (c *Controller) log(format string, args ...any) {
	msg := fmt.Sprintf("[%s] %s", time.Now().Format("15:04:05"), fmt.Sprintf(format, args...))
	select {
	case c.logCh <- msg:
	default:
		// Drop if channel full
	}
}

// Start begins the teleoperation control loop.
func (c *Controller) Start(ctx context.Context) error {
	c.mu.Lock()
	if c.running {
		c.mu.Unlock()
		return fmt.Errorf("already running")
	}
	c.running = true
	c.mu.Unlock()

	// Initialize arms
	if err := c.leader.Disable(ctx); err != nil {
		c.log("Warning: failed to disable leader: %v", err)
	} else {
		c.log("Leader arm: torque disabled (passive mode)")
	}

	if err := c.follower.Enable(ctx); err != nil {
		c.log("Warning: failed to enable follower: %v", err)
	} else {
		c.log("Follower arm: torque enabled")
	}

	c.log("Teleoperation started at %d Hz", c.hz)

	// Control loop
	ticker := time.NewTicker(time.Second / time.Duration(c.hz))
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			c.shutdown()
			return ctx.Err()
		case <-ticker.C:
			c.step(ctx)
		}
	}
}

func (c *Controller) step(ctx context.Context) {
	// Read leader positions
	positions, err := c.leader.ReadPositions(ctx)
	if err != nil {
		c.log("Read error: %v", err)
		c.sendState(State{Error: err, Timestamp: time.Now()})
		return
	}

	// Apply mirroring if enabled (invert shoulder_pan and wrist_roll)
	followerPositions := positions
	if c.mirror {
		followerPositions = make(map[robot.MotorName]float64, len(positions))
		for name, pos := range positions {
			if name == robot.ShoulderPan || name == robot.WristRoll {
				followerPositions[name] = -pos
			} else {
				followerPositions[name] = pos
			}
		}
	}

	// Write to follower
	if err := c.follower.WritePositions(ctx, followerPositions); err != nil {
		c.log("Write error: %v", err)
	}

	// Send state update
	c.sendState(State{
		Positions: positions,
		Timestamp: time.Now(),
	})
}

func (c *Controller) sendState(s State) {
	select {
	case c.stateCh <- s:
	default:
		// Drop old state if channel full, replace with new
		select {
		case <-c.stateCh:
		default:
		}
		c.stateCh <- s
	}
}

func (c *Controller) shutdown() {
	c.mu.Lock()
	c.running = false
	c.mu.Unlock()

	ctx := context.Background()
	if err := c.follower.Disable(ctx); err != nil {
		c.log("Warning: failed to disable follower: %v", err)
	} else {
		c.log("Follower arm: torque disabled")
	}
	c.log("Teleoperation stopped")
}
