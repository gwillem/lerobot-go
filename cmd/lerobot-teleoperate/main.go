package main

import (
	"context"
	"encoding/json"
	"flag"
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

// Config matches the format written by robot-info
type Config struct {
	Leader   PortConfig `json:"leader"`
	Follower PortConfig `json:"follower"`
}

type PortConfig struct {
	Port        string `json:"port"`
	Calibration string `json:"calibration"`
}

const configFile = "lerobot.json"

func loadConfig() (*Config, error) {
	data, err := os.ReadFile(configFile)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
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

type model struct {
	ctrl          *teleop.Controller
	chart         *streamlinechart.Model
	width         int                          // terminal width
	height        int                          // terminal height
	logs          []string                     // last N log messages
	quitting      bool
	lastPositions map[robot.MotorName]float64 // track previous positions to detect movement
}

func (m *model) addLog(msg string) {
	m.logs = append(m.logs, msg)
	if len(m.logs) > maxLogs {
		m.logs = m.logs[len(m.logs)-maxLogs:]
	}
}

// hasMovement checks if any motor position has changed from the last state
func (m *model) hasMovement(positions map[robot.MotorName]float64) bool {
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
func (m *model) chartSize() (width, height int) {
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

func (m *model) resizeChart() {
	w, h := m.chartSize()
	m.chart.Resize(w, h)
}

func initialModel(ctrl *teleop.Controller) model {
	chart := streamlinechart.New(80, 20,
		streamlinechart.WithYRange(-100, 100),
	)

	// Set up data set styles for each motor
	for _, name := range robot.AllMotors() {
		color := motorColors[name]
		style := lipgloss.NewStyle().Foreground(lipgloss.Color(color))
		chart.SetDataSetStyles(string(name), runes.ThinLineStyle, style)
	}

	return model{
		ctrl:  ctrl,
		chart: &chart,
	}
}

func (m model) Init() tea.Cmd {
	// Start listening for state and log updates
	return tea.Batch(
		waitForState(m.ctrl),
		waitForLog(m.ctrl),
	)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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

func (m model) View() string {
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

func main() {
	// Parse command-line flags
	var (
		robotPort  = flag.String("robot.port", "", "Robot serial port (optional if lerobot.json exists)")
		robotID    = flag.String("robot.id", "follower", "Robot ID")
		teleopPort = flag.String("teleop.port", "", "Teleop serial port (optional if lerobot.json exists)")
		teleopID   = flag.String("teleop.id", "leader", "Teleop ID")
		hz         = flag.Int("hz", 60, "Control loop frequency")
		mirror     = flag.Bool("mirror", false, "Mirror mode: invert shoulder_pan and wrist_roll positions")
	)
	flag.String("robot.type", "so101_follower", "Robot type")
	flag.String("teleop.type", "so101_leader", "Teleop type")
	flag.Parse()

	// Try to load config file if ports not specified
	leaderPort := *teleopPort
	leaderCalib := fmt.Sprintf("calibration/%s.json", *teleopID)
	followerPort := *robotPort
	followerCalib := fmt.Sprintf("calibration/%s.json", *robotID)

	if leaderPort == "" || followerPort == "" {
		cfg, err := loadConfig()
		if err != nil {
			fmt.Fprintf(os.Stderr, "No ports specified and cannot load %s: %v\n", configFile, err)
			fmt.Fprintln(os.Stderr, "")
			fmt.Fprintln(os.Stderr, "Run 'go run ./cmd/robot-info' to detect and configure ports,")
			fmt.Fprintln(os.Stderr, "or specify ports manually with --robot.port and --teleop.port")
			os.Exit(1)
		}
		if leaderPort == "" {
			leaderPort = cfg.Leader.Port
			leaderCalib = cfg.Leader.Calibration
		}
		if followerPort == "" {
			followerPort = cfg.Follower.Port
			followerCalib = cfg.Follower.Calibration
		}
		fmt.Printf("Loaded configuration from %s\n", configFile)
	}

	// Create controller
	ctrl, err := teleop.NewController(teleop.Config{
		LeaderPort:    leaderPort,
		LeaderCalib:   leaderCalib,
		FollowerPort:  followerPort,
		FollowerCalib: followerCalib,
		Hz:            *hz,
		Mirror:        *mirror,
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
	p := tea.NewProgram(initialModel(ctrl), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		log.Fatalf("Error running program: %v", err)
	}
}
