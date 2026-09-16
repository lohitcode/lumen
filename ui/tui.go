// Package ui renders the interactive terminal interface for lights: a live
// dashboard with a two-second refresh, keyboard-driven power, brightness, and
// white-temperature controls, and a switcher for jumping between lights.
//
// The sliders animate on a dedicated 60 fps frame loop with time-based
// smoothing, so the motion stays even regardless of tick jitter, and the
// layout is built from fixed-width segments so state changes never shift
// neighboring text.
package ui

import (
	"math"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/lohitcode/lumen/light"
)

// Hooks let the embedder react to in-app actions, e.g. to persist them.
type Hooks struct {
	// OnSwitch is called when the user switches to another light.
	OnSwitch func(light.Light)
	// OnRename is called with the light and its new display name (empty
	// when the user cleared the name).
	OnRename func(l light.Light, name string)
}

// Run starts the terminal UI for the given light and blocks until quit.
// labels holds user-chosen display names keyed by light identity, and hooks
// receive switch/rename events for persistence.
func Run(l light.Light, labels map[string]string, hooks Hooks) error {
	// Alternate screen keeps the TUI separate from the shell's scrollback;
	// mouse reporting captures trackpad/wheel gestures, and the frame rate
	// matches the slider animation loop.
	p := tea.NewProgram(newModel(l, labels, hooks),
		tea.WithAltScreen(), tea.WithMouseAllMotion(), tea.WithFPS(60))
	_, err := p.Run()
	return err
}

const (
	discoverTimeout = 2 * time.Second
	refreshEvery    = 2 * time.Second
	spinnerEvery    = 500 * time.Millisecond
	frameEvery      = time.Second / 60
	// Time constant of the slider glide: smaller is snappier, larger is
	// softer. Exponential smoothing is frame-rate independent, so the
	// motion is identical even when individual frames land late.
	sliderTau = 70 * time.Millisecond
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
type animFrameMsg time.Time

// barState is one animated slider: shown eases toward target every frame.
type barState struct {
	shown  float64 // 0..1 currently displayed
	target float64 // 0..1 where the bar is heading
}

type model struct {
	current       light.Light
	status        light.State
	labels        map[string]string
	hooks         Hooks
	brightnessBar barState
	tempBar       barState
	animRunning   bool
	lastFrame     time.Time
	input         textinput.Model
	renaming      bool
	mode          mode
	found         []light.Light
	pick          int
	cursor        int
	message       string
	err           error
	failures      int
	frame         int
	busy          bool
	quitting      bool
}

func newModel(l light.Light, labels map[string]string, hooks Hooks) model {
	m := model{
		current: l,
		labels:  labels,
		hooks:   hooks,
		message: connectingMessage,
	}
	// Seed with the real state when it is already known so the sliders
	// start filled at the light's current values; the refresh loop takes
	// over from there.
	if state, err := l.State(); err == nil {
		m.status = state
		m.message = ""
		m.setBarTargets()
		m.brightnessBar.shown = m.brightnessBar.target
		m.tempBar.shown = m.tempBar.target
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

func nextAnimFrame() tea.Cmd {
	return tea.Tick(frameEvery, func(t time.Time) tea.Msg { return animFrameMsg(t) })
}

// setBarTargets points both sliders at the current state.
func (m *model) setBarTargets() {
	r := m.current.Ranges()
	m.brightnessBar.target = clamp01(float64(m.status.Brightness) / 100)
	m.tempBar.target = clamp01(float64(m.status.Temp-r.Temp.Min) / float64(rangeSize(r.Temp)))
}

// syncBars updates the slider targets and makes sure the frame loop is
// running so they glide there.
func (m *model) syncBars() tea.Cmd {
	m.setBarTargets()
	return m.ensureAnimFrame()
}

// ensureAnimFrame starts the 60 fps frame loop if it is not already running.
func (m *model) ensureAnimFrame() tea.Cmd {
	if m.animRunning {
		return nil
	}
	m.animRunning = true
	m.lastFrame = time.Now()
	return nextAnimFrame()
}

// stepBars advances the glide using the real elapsed time, so the motion is
// identical whether a frame lands on schedule or a few milliseconds late.
func (m *model) stepBars(now time.Time) {
	dt := now.Sub(m.lastFrame).Seconds()
	if dt <= 0 {
		dt = float64(frameEvery) / float64(time.Second)
	}
	if dt > 0.1 {
		dt = 0.1
	}
	m.lastFrame = now
	blend := 1 - math.Exp(-dt/sliderTau.Seconds())
	m.brightnessBar.shown += (m.brightnessBar.target - m.brightnessBar.shown) * blend
	m.tempBar.shown += (m.tempBar.target - m.tempBar.shown) * blend
}

func (m *model) barsAnimating() bool {
	return math.Abs(m.brightnessBar.shown-m.brightnessBar.target) > 0.0005 ||
		math.Abs(m.tempBar.shown-m.tempBar.target) > 0.0005
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
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

// labelFor returns a light's user-chosen alias, or its device label.
func (m model) labelFor(l light.Light) string {
	if alias, ok := m.labels[identity(l)]; ok && alias != "" {
		return alias
	}
	return l.Label()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch m.mode {
		case modeSearching:
			if msg.String() == "esc" {
				m.mode = modeNormal
				m.message = ""
			}
			return m, nil
		case modePicking:
			return m.updatePicking(msg)
		}

		switch msg.String() {
		case "q", "ctrl+c", "esc":
			m.quitting = true
			return m, tea.Quit
		case "s":
			m.mode = modeSearching
			m.message = ""
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
	case animFrameMsg:
		if !m.animRunning {
			return m, nil
		}
		m.stepBars(time.Time(msg))
		if m.barsAnimating() {
			return m, nextAnimFrame()
		}
		// Settled: snap to the exact target and stop the loop.
		m.brightnessBar.shown = m.brightnessBar.target
		m.tempBar.shown = m.tempBar.target
		m.animRunning = false
		return m, nil
	case statusMsg:
		wasBusy := m.busy
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
		m.message = ""
		if wasBusy {
			// The read raced an in-flight adjustment; the action's own
			// read-back has not landed yet, so don't yank the sliders back.
			return m, nil
		}
		cmd := m.syncBars()
		return m, cmd
	case actionMsg:
		m.busy = false
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.err = nil
		m.message = ""
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

func (m model) updatePicking(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.renaming {
		switch msg.String() {
		case "enter":
			name := strings.TrimSpace(m.input.Value())
			chosen := m.found[m.pick]
			id := identity(chosen)
			if name == "" {
				delete(m.labels, id)
			} else {
				m.labels[id] = name
			}
			if m.hooks.OnRename != nil {
				m.hooks.OnRename(chosen, name)
			}
			m.renaming = false
			m.input.Blur()
			return m, nil
		case "esc":
			m.renaming = false
			m.input.Blur()
			return m, nil
		}
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}

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
	case "n":
		m.renaming = true
		m.input = newRenameInput(m.labels[identity(m.found[m.pick])])
		return m, textinput.Blink
	case "enter":
		chosen := m.found[m.pick]
		m.mode = modeNormal
		if identity(chosen) != identity(m.current) {
			// The header updates to the new light's name — that is the
			// feedback, so the status line stays clean.
			m.current = chosen
			m.message = ""
			if m.hooks.OnSwitch != nil {
				m.hooks.OnSwitch(chosen)
			}
			return m, m.refresh()
		}
	}
	return m, nil
}

// newRenameInput builds the inline editor used to alias a light in the
// switcher: prefilled with any existing alias, focused, no prompt clutter.
func newRenameInput(alias string) textinput.Model {
	in := textinput.New()
	in.Placeholder = "Light name"
	in.CharLimit = 32
	in.Width = 30
	in.Prompt = ""
	in.SetValue(alias)
	in.Focus()
	return in
}

// adjust nudges the selected row's value by delta (brightness, temperature).
// The slider starts gliding immediately for instant feedback, then the
// refresh reconciles with the bulb's real state.
func (m model) adjust(brightness, temp int) (tea.Model, tea.Cmd) {
	r := m.current.Ranges()
	m.busy = true
	if m.cursor == 0 {
		value := r.Brightness.Clamp(m.status.Brightness + brightness)
		m.brightnessBar.target = clamp01(float64(value) / 100)
		return m, tea.Batch(m.ensureAnimFrame(),
			m.change(func(l light.Light) error { return l.SetBrightness(value) }))
	}
	value := r.Temp.Clamp(m.status.Temp + temp)
	m.tempBar.target = clamp01(float64(value-r.Temp.Min) / float64(rangeSize(r.Temp)))
	return m, tea.Batch(m.ensureAnimFrame(),
		m.change(func(l light.Light) error { return l.SetTemp(value) }))
}

func identity(l light.Light) string { return l.Driver() + "/" + l.Address() }
