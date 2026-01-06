package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/hipsterbrown/feetech-servo/feetech"
	"go.bug.st/serial"

	"github.com/gwillem/lerobot/pkg/robot"
)

var (
	headerStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	subHeaderStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("14"))
	successStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	dimStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
)

type SetupCommand struct{}

func (c *SetupCommand) Execute(args []string) error {
	fmt.Println(headerStyle.Render("LeRobot Setup"))
	fmt.Println(dimStyle.Render("━━━━━━━━━━━━━━"))
	fmt.Println()

	// Step 1: Scan for arms
	config := scanForArms()

	// Step 2: Calibrate leader
	fmt.Println()
	fmt.Println(subHeaderStyle.Render("━━━ Calibrating Leader Arm ━━━"))
	fmt.Println()
	calibrateArm(&config.Leader, "leader")

	// Save after leader calibration
	if err := config.Save(); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving config: %v\n", err)
		os.Exit(1)
	}

	// Step 3: Calibrate follower
	fmt.Println()
	fmt.Println(subHeaderStyle.Render("━━━ Calibrating Follower Arm ━━━"))
	fmt.Println()
	calibrateArm(&config.Follower, "follower")

	// Save final config
	if err := config.Save(); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving config: %v\n", err)
		os.Exit(1)
	}

	fmt.Println()
	fmt.Println(dimStyle.Render("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"))
	fmt.Println(successStyle.Render("Setup complete!"))
	fmt.Printf("Configuration saved to %s\n", robot.DefaultConfigFile)
	fmt.Println()
	fmt.Println("Start teleoperation with: " + headerStyle.Render("lerobot teleoperate"))

	return nil
}

func scanForArms() *robot.Config {
	fmt.Println("Scanning for robot arms...")
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
		role := identifyArmWithWiggle(arm, leaderPort == "", followerPort == "")
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
	if leaderPort == "" || followerPort == "" {
		fmt.Println(dimStyle.Render("━━━━━━━━━━━━━━━━━━━━━"))
		if leaderPort == "" {
			fmt.Println("Leader arm not identified.")
		}
		if followerPort == "" {
			fmt.Println("Follower arm not identified.")
		}
		fmt.Println()
		fmt.Println("Both leader and follower are required for teleoperation.")
		os.Exit(1)
	}

	// Display results
	fmt.Println(dimStyle.Render("━━━━━━━━━━━━━━━━━━━━━"))
	fmt.Println(successStyle.Render("Arms identified:"))
	fmt.Printf("  Leader:   %s\n", leaderPort)
	fmt.Printf("  Follower: %s\n", followerPort)

	return &robot.Config{
		Leader: robot.ArmConfig{
			Port: leaderPort,
		},
		Follower: robot.ArmConfig{
			Port: followerPort,
		},
	}
}

func calibrateArm(armConfig *robot.ArmConfig, armName string) {
	fmt.Printf("Calibrating %s arm on %s\n", armName, armConfig.Port)
	fmt.Println()

	// Connect to arm
	bus, servos, err := connectToArm(armConfig.Port)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error connecting to arm: %v\n", err)
		os.Exit(1)
	}
	defer bus.Close()

	// Create servos map by ID
	servoMap := make(map[int]*feetech.Servo)
	for _, s := range servos {
		servoMap[s.ID] = feetech.NewServo(bus, s.ID, s.Model)
	}

	// Disable all servos so user can move arm freely
	ctx := context.Background()
	for _, servo := range servoMap {
		servo.Disable(ctx)
	}

	motors := robot.AllMotors()
	calibration := make(robot.Calibration)

	// Record min/max by tracking while user moves arm
	fmt.Println(subHeaderStyle.Render("Record range of motion"))
	fmt.Println("Move each joint to its minimum AND maximum positions.")
	fmt.Println("Explore the full range of motion for all joints.")
	fmt.Println()

	// Initialize tracking maps
	curPositions := make(map[robot.MotorName]int)
	minPositions := make(map[robot.MotorName]int)
	maxPositions := make(map[robot.MotorName]int)
	for i, motorName := range motors {
		servoID := i + 1
		servo := servoMap[servoID]
		pos, _ := servo.Position(ctx)
		curPositions[motorName] = pos
		minPositions[motorName] = pos
		maxPositions[motorName] = pos
	}

	// Run calibration TUI
	model := newCalibrationModel(motors, servoMap, curPositions, minPositions, maxPositions)
	p := tea.NewProgram(model)
	finalModel, err := p.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error running calibration: %v\n", err)
		os.Exit(1)
	}

	// Get final positions from model
	cm := finalModel.(calibrationModel)
	for _, name := range motors {
		minPositions[name] = cm.minPositions[name]
		maxPositions[name] = cm.maxPositions[name]
	}

	fmt.Println()

	// Build calibration
	for i, motorName := range motors {
		servoID := i + 1
		calibration[motorName] = robot.MotorCalibration{
			ID:       servoID,
			RangeMin: minPositions[motorName],
			RangeMax: maxPositions[motorName],
		}
	}

	armConfig.Calibration = calibration
	fmt.Println()
	fmt.Printf("%s arm calibrated.\n", strings.Title(armName))
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

func identifyArmWithWiggle(arm armInfo, needLeader, needFollower bool) string {
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

	fmt.Printf("\n  Wiggling arm on %s...\n", arm.port)

	// Wiggle: single gentle, slow movement
	wiggleAmount := 30
	moveTimeMs := 500
	servo.SetPositionWithTime(ctx, originalPos+wiggleAmount, moveTimeMs)
	time.Sleep(time.Duration(moveTimeMs+100) * time.Millisecond)
	servo.SetPositionWithTime(ctx, originalPos-wiggleAmount, moveTimeMs)
	time.Sleep(time.Duration(moveTimeMs+100) * time.Millisecond)

	// Return to original position
	servo.SetPositionWithTime(ctx, originalPos, moveTimeMs)
	time.Sleep(time.Duration(moveTimeMs+100) * time.Millisecond)

	// Disable torque
	servo.Disable(ctx)

	// Build options based on what's still needed
	var options []huh.Option[string]
	if needLeader {
		options = append(options, huh.NewOption("Leader (the one you move by hand)", "leader"))
	}
	if needFollower {
		options = append(options, huh.NewOption("Follower (the one that follows)", "follower"))
	}
	options = append(options, huh.NewOption("Skip this arm", "skip"))

	// Ask user which arm this is
	var role string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title(fmt.Sprintf("Which arm is on %s?", arm.port)).
				Description("The arm that just wiggled").
				Options(options...).
				Value(&role),
		),
	)

	if err := form.Run(); err != nil {
		fmt.Println()
		os.Exit(0)
	}

	if role == "skip" {
		return ""
	}

	return role
}

func connectToArm(port string) (*feetech.Bus, []feetech.FoundServo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	bus, err := feetech.NewBus(feetech.BusConfig{
		Port:     port,
		BaudRate: 1_000_000,
		Protocol: feetech.ProtocolSTS,
		Timeout:  100 * time.Millisecond,
	})
	if err != nil {
		return nil, nil, err
	}

	servos, err := bus.Scan(ctx, 1, 6)
	if err != nil {
		bus.Close()
		return nil, nil, err
	}

	if !isSOArm(servos) {
		bus.Close()
		return nil, nil, fmt.Errorf("not an SO-101 arm (expected 6 servos with IDs 1-6)")
	}

	return bus, servos, nil
}

func waitForUser(prompt string) {
	fmt.Println(prompt)

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("").
				Affirmative("Continue").
				Negative("").
				Value(new(bool)),
		),
	)
	if err := form.Run(); err != nil {
		fmt.Println()
		os.Exit(0)
	}
}

// Calibration TUI model
type calibrationModel struct {
	motors       []robot.MotorName
	servoMap     map[int]*feetech.Servo
	curPositions map[robot.MotorName]int
	minPositions map[robot.MotorName]int
	maxPositions map[robot.MotorName]int
	quitting     bool
}

type tickMsg time.Time

func newCalibrationModel(
	motors []robot.MotorName,
	servoMap map[int]*feetech.Servo,
	curPositions, minPositions, maxPositions map[robot.MotorName]int,
) calibrationModel {
	return calibrationModel{
		motors:       motors,
		servoMap:     servoMap,
		curPositions: curPositions,
		minPositions: minPositions,
		maxPositions: maxPositions,
	}
}

func (m calibrationModel) Init() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m calibrationModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter", "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		}

	case tickMsg:
		// Read positions from servos
		ctx := context.Background()
		for i, motorName := range m.motors {
			servoID := i + 1
			servo := m.servoMap[servoID]
			pos, err := servo.Position(ctx)
			if err != nil {
				continue
			}
			m.curPositions[motorName] = pos
			if pos < m.minPositions[motorName] {
				m.minPositions[motorName] = pos
			}
			if pos > m.maxPositions[motorName] {
				m.maxPositions[motorName] = pos
			}
		}
		return m, tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
			return tickMsg(t)
		})
	}

	return m, nil
}

func (m calibrationModel) View() string {
	if m.quitting {
		return ""
	}

	var sb strings.Builder

	// Table styles
	tableHeaderStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12")).Padding(0, 1)
	tableMotorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("14")).Padding(0, 1)
	tableCellStyle := lipgloss.NewStyle().Padding(0, 1)
	tableCurrentStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Padding(0, 1)
	tableRangeGoodStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Padding(0, 1)
	tableRangeLowStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Padding(0, 1)

	rows := make([][]string, 0, len(m.motors))
	ranges := make([]int, 0, len(m.motors))
	for _, motorName := range m.motors {
		rangeSize := m.maxPositions[motorName] - m.minPositions[motorName]
		ranges = append(ranges, rangeSize)
		rows = append(rows, []string{
			string(motorName),
			fmt.Sprintf("%d", m.curPositions[motorName]),
			fmt.Sprintf("%d", m.minPositions[motorName]),
			fmt.Sprintf("%d", m.maxPositions[motorName]),
			fmt.Sprintf("%d", rangeSize),
		})
	}

	t := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(dimStyle).
		Headers("Motor", "Current", "Min", "Max", "Range").
		Rows(rows...).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == table.HeaderRow {
				return tableHeaderStyle
			}
			switch col {
			case 0:
				return tableMotorStyle
			case 1:
				return tableCurrentStyle
			case 4:
				if row >= 0 && row < len(ranges) && ranges[row] > 500 {
					return tableRangeGoodStyle
				}
				return tableRangeLowStyle
			default:
				return tableCellStyle
			}
		})

	sb.WriteString(t.Render())
	sb.WriteString("\n\n")
	sb.WriteString(dimStyle.Render("Press Enter when done"))

	return sb.String()
}
