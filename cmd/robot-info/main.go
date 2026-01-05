package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/hipsterbrown/feetech-servo/feetech"
	"go.bug.st/serial"
)

// Config is saved to lerobot.json for use by lerobot-teleoperate
type Config struct {
	Leader   PortConfig `json:"leader"`
	Follower PortConfig `json:"follower"`
}

type PortConfig struct {
	Port        string `json:"port"`
	Calibration string `json:"calibration"`
}

const configFile = "lerobot.json"

func main() {
	fmt.Println("🤖 LeRobot Port Scanner")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()

	// Find all ports with SO-101 arms
	arms := findArms()

	if len(arms) == 0 {
		fmt.Println("No SO-101 arms found.")
		fmt.Println("Make sure your arms are connected and powered on.")
		os.Exit(1)
	}

	fmt.Printf("Found %d arm(s). Let's identify them...\n\n", len(arms))

	// Identify each arm by wiggling it
	var leaderPort, followerPort string

	for _, arm := range arms {
		role := identifyArmWithWiggle(arm)
		switch role {
		case "leader":
			leaderPort = arm.port
		case "follower":
			followerPort = arm.port
		}

		// If we have both, we can stop
		if leaderPort != "" && followerPort != "" {
			break
		}
	}

	fmt.Println()

	// Check what we found
	if leaderPort == "" && followerPort == "" {
		fmt.Println("No arms were identified.")
		os.Exit(1)
	}

	// Display results
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("Configuration:")
	if leaderPort != "" {
		fmt.Printf("  Leader:   %s\n", leaderPort)
	} else {
		fmt.Println("  Leader:   (not found)")
	}
	if followerPort != "" {
		fmt.Printf("  Follower: %s\n", followerPort)
	} else {
		fmt.Println("  Follower: (not found)")
	}
	fmt.Println()

	// Save config if we have both
	if leaderPort != "" && followerPort != "" {
		config := Config{
			Leader: PortConfig{
				Port:        leaderPort,
				Calibration: "calibration/leader.json",
			},
			Follower: PortConfig{
				Port:        followerPort,
				Calibration: "calibration/follower.json",
			},
		}

		if err := saveConfig(config); err != nil {
			fmt.Printf("Error saving config: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("✓ Configuration saved to %s\n", configFile)
		fmt.Println()
		fmt.Println("Run teleoperation with:")
		fmt.Println("  go run ./cmd/lerobot-teleoperate")
	} else {
		fmt.Println("⚠ Both leader and follower are required for teleoperation.")
		fmt.Println("  Connect both arms and run this command again.")
	}
}

type armInfo struct {
	port   string
	servos []feetech.FoundServo
	bus    *feetech.Bus
}

func findArms() []armInfo {
	ports, err := serial.GetPortsList()
	if err != nil {
		fmt.Printf("Error listing ports: %v\n", err)
		return nil
	}

	var arms []armInfo

	for _, port := range ports {
		// Skip Bluetooth ports on macOS
		if strings.Contains(port, "Bluetooth") {
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)

		bus, err := feetech.NewBus(feetech.BusConfig{
			Port:     port,
			BaudRate: 1_000_000,
			Protocol: feetech.ProtocolSTS,
			Timeout:  100 * time.Millisecond,
		})
		if err != nil {
			cancel()
			continue
		}

		// Scan for servos with IDs 1-6 (SO-101 arm configuration)
		servos, err := bus.Scan(ctx, 1, 6)
		cancel()

		if err != nil {
			bus.Close()
			continue
		}

		// Check if it's an SO-101 (6 servos with IDs 1-6)
		if isSOArm(servos) {
			fmt.Printf("  Found SO-101 arm on %s\n", port)
			arms = append(arms, armInfo{
				port:   port,
				servos: servos,
				bus:    bus,
			})
		} else {
			bus.Close()
		}
	}

	return arms
}

func isSOArm(servos []feetech.FoundServo) bool {
	if len(servos) != 6 {
		return false
	}

	ids := make(map[int]bool)
	for _, s := range servos {
		ids[s.ID] = true
	}

	for i := 1; i <= 6; i++ {
		if !ids[i] {
			return false
		}
	}

	return true
}

func identifyArmWithWiggle(arm armInfo) string {
	defer arm.bus.Close()

	ctx := context.Background()

	// Find servo ID 1 (shoulder_pan) for wiggling
	var servo *feetech.Servo
	for _, s := range arm.servos {
		if s.ID == 1 {
			servo = feetech.NewServo(arm.bus, s.ID, s.Model)
			break
		}
	}

	if servo == nil {
		return ""
	}

	// Read current position
	originalPos, err := servo.Position(ctx)
	if err != nil {
		fmt.Printf("  Error reading position: %v\n", err)
		return ""
	}

	// Enable torque for wiggle
	if err := servo.Enable(ctx); err != nil {
		fmt.Printf("  Error enabling servo: %v\n", err)
		return ""
	}

	fmt.Printf("\n  ⟳ Wiggling arm on %s...\n", arm.port)

	// Wiggle: move ±50 steps a few times
	wiggleAmount := 100
	for i := 0; i < 3; i++ {
		servo.SetPosition(ctx, originalPos+wiggleAmount)
		time.Sleep(150 * time.Millisecond)
		servo.SetPosition(ctx, originalPos-wiggleAmount)
		time.Sleep(150 * time.Millisecond)
	}

	// Return to original position
	servo.SetPosition(ctx, originalPos)
	time.Sleep(100 * time.Millisecond)

	// Disable torque
	servo.Disable(ctx)

	// Ask user which arm this is
	var role string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title(fmt.Sprintf("Which arm is on %s?", arm.port)).
				Description("The arm that just wiggled").
				Options(
					huh.NewOption("Leader (the one you move by hand)", "leader"),
					huh.NewOption("Follower (the one that follows)", "follower"),
					huh.NewOption("Skip this arm", "skip"),
				).
				Value(&role),
		),
	)

	if err := form.Run(); err != nil {
		return ""
	}

	if role == "skip" {
		return ""
	}

	return role
}

func saveConfig(config Config) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configFile, data, 0644)
}
