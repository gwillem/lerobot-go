package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/NimbleMarkets/ntcharts/canvas/runes"
	"github.com/NimbleMarkets/ntcharts/linechart/streamlinechart"

	"github.com/gwillem/lerobot/pkg/robot"
	"github.com/gwillem/lerobot/pkg/teleop"
)

type TeleoperateCommand struct {
	Hz     int  `long:"hz" default:"60" description:"Control loop frequency"`
	Mirror bool `long:"mirror" description:"Mirror mode: invert shoulder_pan and wrist_roll positions"`
}

const (
	headerHeight = 2 // title + blank line
	legendHeight = 2 // legend row + blank
	footerHeight = 7 // log box height
	maxLogs      = 5 // number of log messages to show
	borderSize   = 2 // chart border
)

// Motor colors - distinct colors for each motor
var motorColors = map[robot.MotorName]string{
	robot.ShoulderPan:  "196", // red
	robot.ShoulderLift: "208", // orange
	robot.ElbowFlex:    "226", // yellow
	robot.WristFlex:    "46",  // green
	robot.WristRoll:    "51",  // cyan
	robot.Gripper:      "201", // magenta
}

var (
	titleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	chartStyle  = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("240"))
	statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
)

type teleopModel struct {
	ctrl          *teleop.Controller
	chart         *streamlinechart.Model
	width         int                          // terminal width
	height        int                          // terminal height
	logs          []string                     // last N log messages
	quitting      bool
	lastPositions map[robot.MotorName]float64 // track previous positions to detect movement
}

func (m *teleopModel) addLog(msg string) {
	m.logs = append(m.logs, msg)
	if len(m.logs) > maxLogs {
		m.logs = m.logs[len(m.logs)-maxLogs:]
	}
}

// hasMovement checks if any motor position has changed from the last state
func (m *teleopModel) hasMovement(positions map[robot.MotorName]float64) bool {
	if m.lastPositions == nil {
		return true // first reading, consider it movement
	}
	for name, pos := range positions {
		if lastPos, ok := m.lastPositions[name]; !ok || pos != lastPos {
			return true
		}
	}
	return false
}

// Messages from the controller
type stateMsg teleop.State
type logMsg string

func waitForState(ctrl *teleop.Controller) tea.Cmd {
	return func() tea.Msg {
		return stateMsg(<-ctrl.States())
	}
}

func waitForLog(ctrl *teleop.Controller) tea.Cmd {
	return func() tea.Msg {
		return logMsg(<-ctrl.Logs())
	}
}

// chartSize calculates the size of the chart based on terminal dimensions
func (m *teleopModel) chartSize() (width, height int) {
	if m.width == 0 || m.height == 0 {
		return 80, 20 // default size before we know terminal size
	}
	width = m.width - borderSize - 2
	if width < 40 {
		width = 40
	}
	height = m.height - headerHeight - legendHeight - footerHeight - borderSize
	if height < 10 {
		height = 10
	}
	return width, height
}

func (m *teleopModel) resizeChart() {
	w, h := m.chartSize()
	m.chart.Resize(w, h)
}

func initialTeleopModel(ctrl *teleop.Controller) teleopModel {
	chart := streamlinechart.New(80, 20,
		streamlinechart.WithYRange(-100, 100),
	)

	// Set up data set styles for each motor
	for _, name := range robot.AllMotors() {
		color := motorColors[name]
		style := lipgloss.NewStyle().Foreground(lipgloss.Color(color))
		chart.SetDataSetStyles(string(name), runes.ThinLineStyle, style)
	}

	return teleopModel{
		ctrl:  ctrl,
		chart: &chart,
	}
}

func (m teleopModel) Init() tea.Cmd {
	// Start listening for state and log updates
	return tea.Batch(
		waitForState(m.ctrl),
		waitForLog(m.ctrl),
	)
}

func (m teleopModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resizeChart()
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		}

	case stateMsg:
		state := teleop.State(msg)
		if state.Positions != nil {
			// Only update chart if there's movement (freeze when idle)
			if m.hasMovement(state.Positions) {
				for name, pos := range state.Positions {
					m.chart.PushDataSet(string(name), pos)
				}
				m.chart.DrawAll()
				m.lastPositions = state.Positions
			}
		}
		return m, waitForState(m.ctrl)

	case logMsg:
		m.addLog(string(msg))
		return m, waitForLog(m.ctrl)
	}

	return m, nil
}

func (m teleopModel) View() string {
	if m.quitting {
		return "Teleoperation stopped.\n"
	}

	var sb strings.Builder

	// Header
	sb.WriteString(titleStyle.Render("LeRobot Teleoperate"))
	sb.WriteString(fmt.Sprintf(" - %d Hz", m.ctrl.Hz()))
	if m.width > 0 {
		sb.WriteString(statusStyle.Render(fmt.Sprintf("  [%dx%d]", m.width, m.height)))
	}
	sb.WriteString("\n\n")

	// Chart
	sb.WriteString(chartStyle.Render(m.chart.View()))
	sb.WriteString("\n")

	// Legend
	sb.WriteString(renderLegend())
	sb.WriteString("\n")

	// Log box
	logStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240")).
		Width(m.width - 4).
		Foreground(lipgloss.Color("9")) // bright red

	var logLines string
	if len(m.logs) == 0 {
		logLines = statusStyle.Render("Press 'q' to quit")
	} else {
		logLines = strings.Join(m.logs, "\n")
	}
	sb.WriteString(logStyle.Render(logLines))
	sb.WriteString("\n")

	return sb.String()
}

func renderLegend() string {
	var items []string
	for _, name := range robot.AllMotors() {
		color := motorColors[name]
		colorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Bold(true)
		item := colorStyle.Render("━━") + " " + string(name)
		items = append(items, item)
	}
	return strings.Join(items, "  ")
}

func (c *TeleoperateCommand) Execute(args []string) error {
	// Load config
	cfg, err := robot.LoadConfig()
	if err != nil {
		fmt.Fprintln(os.Stderr, "No configuration found. Run 'lerobot setup' first.")
		os.Exit(1)
	}

	// Check ports are configured
	if cfg.Leader.Port == "" || cfg.Follower.Port == "" {
		fmt.Fprintln(os.Stderr, "Arms not configured. Run 'lerobot setup' first.")
		os.Exit(1)
	}

	// Check calibration
	if !cfg.Leader.IsCalibrated() || !cfg.Follower.IsCalibrated() {
		fmt.Fprintln(os.Stderr, "Arms not calibrated. Run 'lerobot setup' first.")
		os.Exit(1)
	}

	fmt.Printf("Loaded configuration from %s\n", robot.DefaultConfigFile)

	// Create controller
	ctrl, err := teleop.NewController(teleop.Config{
		LeaderPort:          cfg.Leader.Port,
		LeaderCalibration:   cfg.Leader.Calibration,
		FollowerPort:        cfg.Follower.Port,
		FollowerCalibration: cfg.Follower.Calibration,
		Hz:                  c.Hz,
		Mirror:              c.Mirror,
	})
	if err != nil {
		log.Fatalf("Failed to create controller: %v", err)
	}
	defer ctrl.Close()

	// Start controller in background
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		if err := ctrl.Start(ctx); err != nil && err != context.Canceled {
			log.Printf("Controller error: %v", err)
		}
	}()

	// Run TUI
	p := tea.NewProgram(initialTeleopModel(ctrl), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		log.Fatalf("Error running program: %v", err)
	}

	return nil
}
