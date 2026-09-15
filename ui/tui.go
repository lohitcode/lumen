// Package ui renders the interactive terminal interface for a WiZ light:
// live state with a two-second refresh, and keyboard-driven power, brightness,
// and white-temperature controls.
package ui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/lohitcode/study-light/wiz"
)

// Run starts the terminal UI for the given light and blocks until quit.
func Run(d wiz.Device) error {
	// Alternate screen keeps the TUI separate from the shell's scrollback; mouse
	// reporting also captures trackpad/wheel gestures instead of scrolling it.
	p := tea.NewProgram(newModel(d), tea.WithAltScreen(), tea.WithMouseAllMotion())
	_, err := p.Run()
	return err
}

type statusMsg struct {
	reply wiz.Response
	err   error
}
type actionMsg struct {
	reply wiz.Response
	err   error
}
type tickMsg time.Time

type model struct {
	device   wiz.Device
	status   wiz.Response
	cursor   int
	message  string
	err      error
	busy     bool
	quitting bool
}

func newModel(d wiz.Device) model {
	return model{device: d, status: d.Info, message: "Connecting to Study Light…"}
}

func (m model) Init() tea.Cmd { return tea.Batch(m.refresh(), nextTick()) }

func nextTick() tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m model) refresh() tea.Cmd {
	return func() tea.Msg {
		reply, err := wiz.GetState(m.device.Host)
		return statusMsg{reply, err}
	}
}

func (m model) change(params map[string]any) tea.Cmd {
	return func() tea.Msg {
		reply, err := wiz.SetLight(m.device.Host, params)
		return actionMsg{reply, err}
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			m.quitting = true
			return m, tea.Quit
		case "o", " ":
			m.busy = true
			state := !boolValue(m.status.Result, "state")
			return m, m.change(map[string]any{"state": state})
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < 1 {
				m.cursor++
			}
		case "left", "h":
			if m.cursor == 0 {
				return m.adjust("dimming", -5, 10, 100)
			}
			if m.cursor == 1 {
				return m.adjust("temp", -100, 2700, 6500)
			}
		case "right", "l":
			if m.cursor == 0 {
				return m.adjust("dimming", 5, 10, 100)
			}
			if m.cursor == 1 {
				return m.adjust("temp", 100, 2700, 6500)
			}
		}
	case statusMsg:
		m.busy = false
		m.err = msg.err
		if msg.err == nil {
			m.status = msg.reply
			m.message = "Live status · refreshed just now"
		}
		return m, nil
	case actionMsg:
		m.busy = false
		m.err = msg.err
		if msg.err == nil && msg.reply.Error != nil {
			m.err = msg.reply.Error
		}
		if m.err == nil {
			m.message = "Applied to Study Light"
			return m, m.refresh()
		}
	case tickMsg:
		return m, tea.Batch(m.refresh(), nextTick())
	}
	return m, nil
}

func (m model) adjust(field string, delta, min, max int) (tea.Model, tea.Cmd) {
	value := intValue(m.status.Result, field) + delta
	if value < min {
		value = min
	}
	if value > max {
		value = max
	}
	m.busy = true
	return m, m.change(map[string]any{field: value})
}
