// Package ui renders the interactive terminal interface for lights: a live
// dashboard with a two-second refresh, keyboard-driven power, brightness, and
// white-temperature controls, and a switcher for jumping between lights.
//
// The layout is built entirely from fixed-width segments so that state
// changes never shift neighboring text — the view stays still and only
// values change.
package ui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/lohitcode/study-light/light"
)

// Run starts the terminal UI for the given light and blocks until quit.
// onSwitch is called whenever the user switches to another light, so callers
// can persist the choice.
func Run(l light.Light, onSwitch func(light.Light)) error {
	// Alternate screen keeps the TUI separate from the shell's scrollback; mouse
	// reporting also captures trackpad/wheel gestures instead of scrolling it.
	p := tea.NewProgram(newModel(l, onSwitch), tea.WithAltScreen(), tea.WithMouseAllMotion())
	_, err := p.Run()
	return err
}

const (
	discoverTimeout = 2 * time.Second
	refreshEvery    = 2 * time.Second
	spinnerEvery    = 500 * time.Millisecond
	// A single missed UDP read is unremarkable; only report an error once
	// several reads in a row have failed, so the status line doesn't flash.
	maxReadFailures = 2
)

const connectingMessage = "Connecting…"

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

type mode int

const (
	modeNormal mode = iota
	modeSearching
	modePicking
)

type statusMsg struct {
	state light.State
	err   error
}
type actionMsg struct{ err error }
type lightsMsg struct {
	lights []light.Light
	err    error
}
type tickMsg time.Time
type spinnerTickMsg time.Time

type model struct {
	current  light.Light
	status   light.State
	onSwitch func(light.Light)
	mode     mode
	found    []light.Light
	pick     int
	cursor   int
	message  string
	err      error
	failures int
	frame    int
	busy     bool
	quitting bool
}

func newModel(l light.Light, onSwitch func(light.Light)) model {
	m := model{current: l, onSwitch: onSwitch, message: connectingMessage}
	// Seed with the real state when it is already known so the first paint
	// never flashes wrong values; the refresh loop takes over from there.
	if state, err := l.State(); err == nil {
		m.status = state
		m.message = "Connected"
	}
	return m
}

func (m model) Init() tea.Cmd {
	return tea.Batch(m.refresh(), nextTick(), nextSpinner())
}

func nextTick() tea.Cmd {
	return tea.Tick(refreshEvery, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func nextSpinner() tea.Cmd {
	return tea.Tick(spinnerEvery, func(t time.Time) tea.Msg { return spinnerTickMsg(t) })
}

func (m model) refresh() tea.Cmd {
	return func() tea.Msg {
		state, err := m.current.State()
		return statusMsg{state, err}
	}
}

func (m model) change(apply func(light.Light) error) tea.Cmd {
	return func() tea.Msg { return actionMsg{apply(m.current)} }
}

func (m model) discover() tea.Cmd {
	return func() tea.Msg {
		lights, err := light.DiscoverAll(discoverTimeout)
		return lightsMsg{lights, err}
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch m.mode {
		case modeSearching:
			if msg.String() == "esc" {
				m.mode = modeNormal
				m.message = "Search cancelled"
			}
			return m, nil
		case modePicking:
			switch msg.String() {
			case "esc", "q":
				m.mode = modeNormal
				return m, nil
			case "up", "k":
				if m.pick > 0 {
					m.pick--
				}
			case "down", "j":
				if m.pick < len(m.found)-1 {
					m.pick++
				}
			case "enter":
				chosen := m.found[m.pick]
				m.mode = modeNormal
				if identity(chosen) != identity(m.current) {
					m.current = chosen
					m.message = "Switched to " + chosen.Label()
					if m.onSwitch != nil {
						m.onSwitch(chosen)
					}
					return m, m.refresh()
				}
			}
			return m, nil
		}

		switch msg.String() {
		case "q", "ctrl+c", "esc":
			m.quitting = true
			return m, tea.Quit
		case "s":
			m.mode = modeSearching
			m.message = "Searching for lights…"
			return m, m.discover()
		case "o", " ":
			m.busy = true
			on := !m.status.On
			return m, m.change(func(l light.Light) error { return l.SetPower(on) })
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < 1 {
				m.cursor++
			}
		case "left", "h":
			return m.adjust(-5, -100)
		case "right", "l":
			return m.adjust(5, 100)
		}
	case spinnerTickMsg:
		// Only animate while there is something to wait for; a steady dot
		// reads calmer when everything is already live.
		if m.busy || m.mode == modeSearching {
			m.frame = (m.frame + 1) % len(spinnerFrames)
		}
		return m, nextSpinner()
	case statusMsg:
		m.busy = false
		if msg.err != nil {
			m.failures++
			if m.failures >= maxReadFailures {
				m.err = msg.err
			}
			return m, nil
		}
		m.failures = 0
		m.err = nil
		m.status = msg.state
		if m.message == connectingMessage {
			m.message = "Connected"
		}
		return m, nil
	case actionMsg:
		m.busy = false
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.err = nil
		m.message = "Applied to " + m.current.Label()
		return m, m.refresh()
	case lightsMsg:
		if m.mode != modeSearching {
			return m, nil // a search that was cancelled mid-flight
		}
		if msg.err != nil {
			m.mode = modeNormal
			m.err = msg.err
			return m, nil
		}
		m.found = msg.lights
		m.pick = 0
		m.mode = modePicking
		for i, l := range m.found {
			if identity(l) == identity(m.current) {
				m.pick = i
			}
		}
		return m, nil
	case tickMsg:
		return m, tea.Batch(m.refresh(), nextTick())
	}
	return m, nil
}

// adjust nudges the selected row's value by delta (brightness, temperature)
// and sends the clamped result to the light.
func (m model) adjust(brightness, temp int) (tea.Model, tea.Cmd) {
	ranges := m.current.Ranges()
	m.busy = true
	if m.cursor == 0 {
		value := ranges.Brightness.Clamp(m.status.Brightness + brightness)
		return m, m.change(func(l light.Light) error { return l.SetBrightness(value) })
	}
	value := ranges.Temp.Clamp(m.status.Temp + temp)
	return m, m.change(func(l light.Light) error { return l.SetTemp(value) })
}

func identity(l light.Light) string { return l.Driver() + "/" + l.Address() }
