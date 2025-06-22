// Package main implements a terminal dashboard for monitoring multiple instances
// with live TPS metrics and log viewing capabilities.
package main

import (
	"flag"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
)

type instance struct {
	id      int
	tps     int
	pending int
	logChan chan string
	logBuf  []string
}

type model struct {
	inst     []instance
	view     string
	activeID int
	input    textinput.Model
	tick     time.Time
	width    int
	height   int
	errMsg   string
	errTimer int
}

type tickMsg time.Time
type logMsg struct {
	id   int
	line string
}

var (
	headerStyle = lipgloss.NewStyle().Bold(true).Underline(true)
	boldStyle   = lipgloss.NewStyle().Bold(true)
	errorStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	logBus      = make(chan logMsg, 256) // send-only for producers
)

func main() {
	var instFlag = flag.Int("instances", 0, "number of dummy instances")
	flag.Parse()

	n := *instFlag
	if n <= 0 {
		n = rand.Intn(20) + 10
	}

	instances := make([]instance, n)
	for i := 0; i < n; i++ {
		instances[i] = instance{
			id:      i,
			tps:     rand.Intn(50) + 10,
			pending: rand.Intn(20),
			logChan: make(chan string, 100),
			logBuf:  make([]string, 0, 100),
		}
	}

	ti := textinput.New()
	ti.Placeholder = "type number and Enter"
	ti.Focus()
	ti.CharLimit = 4
	ti.Width = 10

	m := model{
		inst:  instances,
		view:  "dash",
		input: ti,
		tick:  time.Now(),
	}

	// Spawn goroutines to simulate instance activity
	for i := range instances {
		go func(id int) {
			randSleep := func() { time.Sleep(time.Duration(400+rand.Intn(400)) * time.Millisecond) }
			sample := []string{
				"Getting latest blockhash...",
				"Got blockhash: %s",
				"→ Transaction: %s… to %s…",
				"Batch sent: %d/%d successful",
			}

			for {
				randSleep()

				instances[id].tps = rand.Intn(50) + 10
				instances[id].pending = rand.Intn(20)
				// Generate different log message types
				switch n := rand.Intn(4); n {
				case 0:
					logBus <- logMsg{id, fmt.Sprintf("[Instance %d] %s", id, sample[n])}
				case 1:
					bh := randSeq(6)
					logBus <- logMsg{id, fmt.Sprintf("[Instance %d] %s", id, fmt.Sprintf(sample[n], bh))}
				case 2:
					sig := randSeq(7)
					dest := randSeq(5)
					logBus <- logMsg{id, fmt.Sprintf("[Instance %d] %s", id, fmt.Sprintf(sample[n], sig, dest))}
				case 3:
					good := 30
					total := 30
					logBus <- logMsg{id, fmt.Sprintf("[Instance %d] %s", id, fmt.Sprintf(sample[n], good, total))}
				}
			}
		}(i)
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v\n", err)
	}

	for i := range instances {
		close(instances[i].logChan)
	}
}


// randHash generates a random hash-like string of specified length.
func randHash(length int) string {
	const charset = "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

// randSeq generates a random alphanumeric sequence.
func randSeq(n int) string {
	const letters = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

// Init returns initial commands for the Bubble Tea program.
func (m model) Init() tea.Cmd {
	return tea.Batch(
		tickCmd(),
		listenLogs(),
		m.input.Focus(),
	)
}

// tickCmd creates a command that fires every second.
func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// listenLogs waits for log messages and converts them to tea.Msg.
func listenLogs() tea.Cmd {
	return func() tea.Msg {
		return <-logBus
	}
}


// Update handles incoming messages and updates the model state.
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tickMsg:
		m.tick = time.Time(msg)
		if m.errTimer > 0 {
			m.errTimer--
			if m.errTimer == 0 {
				m.errMsg = ""
			}
		}
		cmds = append(cmds, tickCmd())

	case logMsg:
		id := msg.id
		if id >= 0 && id < len(m.inst) {
			inst := &m.inst[id]
			inst.logBuf = append(inst.logBuf, time.Now().Format("15:04:05 ") + msg.line)
			if len(inst.logBuf) > 100 {
				inst.logBuf = inst.logBuf[1:]
			}
		}
		return m, listenLogs()

	case tea.KeyMsg:
		switch msg.String() {
		case "q":
			return m, tea.Quit
		case "esc":
			if m.view == "log" {
				m.view = "dash"
				m.errMsg = ""
			}
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	cmds = append(cmds, cmd)
	if m.view == "dash" && m.input.Value() != "" {
		if key, ok := msg.(tea.KeyMsg); ok && key.String() == "enter" {
			idStr := strings.TrimSpace(m.input.Value())
			id, err := strconv.Atoi(idStr)
			if err != nil || id < 0 || id >= len(m.inst) {
				m.errMsg = "invalid ID"
				m.input.SetValue("")
			} else {
				m.view = "log"
				m.activeID = id
				m.input.SetValue("")
				m.errMsg = ""
			}
		}
	}

	return m, tea.Batch(cmds...)
}

// View renders the current state as a string for display.
func (m model) View() string {
	if m.view == "dash" {
		return m.dashboardView()
	}
	return m.logView()
}

func (m model) dashboardView() string {
	var totalTPS int
	for _, inst := range m.inst {
		totalTPS += inst.tps
	}
	avgTPS := 0
	if len(m.inst) > 0 {
		avgTPS = totalTPS / len(m.inst)
	}

	var b strings.Builder
	b.WriteString(headerStyle.Render(fmt.Sprintf("%d instances live · %d TPS avg", len(m.inst), avgTPS)))
	b.WriteString("\n\n")

	b.WriteString(boldStyle.Render("  ID   TPS   Pending"))
	b.WriteString("\n")

	// Calculate visible area accounting for header/footer
	visibleRows := m.height - 8
	if m.errMsg != "" {
		visibleRows--
	}

	startIdx := 0
	endIdx := len(m.inst)
	if endIdx > visibleRows && visibleRows > 0 {
		endIdx = startIdx + visibleRows
	}

	for i := startIdx; i < endIdx && i < len(m.inst); i++ {
		inst := m.inst[i]
		b.WriteString(fmt.Sprintf("%3d %5d %8d\n", inst.id, inst.tps, inst.pending))
	}

	if endIdx < len(m.inst) {
		b.WriteString(fmt.Sprintf("  ... %d more instances ...\n", len(m.inst)-endIdx))
	}

	b.WriteString("\nSelect instance > ")
	b.WriteString(m.input.View())

	if m.errMsg != "" {
		b.WriteString("\n")
		b.WriteString(errorStyle.Render(m.errMsg))
	}

	return b.String()
}

func (m model) logView() string {
	var b strings.Builder
	b.WriteString(headerStyle.Render(fmt.Sprintf("Logs — instance %d   (ESC to back)", m.activeID)))
	b.WriteString("\n\n")

	if m.activeID >= 0 && m.activeID < len(m.inst) {
		inst := m.inst[m.activeID]
		// Show only last 20 log lines
		start := 0
		if len(inst.logBuf) > 20 {
			start = len(inst.logBuf) - 20
		}
		lines := strings.Join(inst.logBuf[start:], "\n")
		b.WriteString(lines)
		b.WriteString("\n")
	}

	return b.String()
}