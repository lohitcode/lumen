// Package ui renders the interactive terminal interface for lights: a live
// dashboard with a two-second refresh, keyboard-driven power, brightness, and
// white-temperature controls, and a switcher for jumping between lights.
//
// The sliders are native bubbles/progress bars with spring animation at
// 60 fps, and the layout is built from fixed-width segments so state changes
// never shift neighboring text.
package ui

import (
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
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
	// mouse reporting captures trackpad/wheel gestures, and the explicit
	// frame rate keeps the spring-animated sliders at the full 60 fps.
	p := tea.NewProgram(newModel(l, labels, hooks),
		tea.WithAltScreen(), tea.WithMouseAllMotion(), tea.WithFPS(60))
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
	current       light.Light
	status        light.State
	labels        map[string]string
	hooks         Hooks
	brightnessBar progress.Model
	tempBar       progress.Model
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

// newBrightnessBar builds the spring-animated brightness slider.
func newBrightnessBar() progress.Model {
	b := progress.New(
		progress.WithWidth(meterWidth),
		progress.WithFillCharacters('█', '█'),
		progress.WithSolidFill("#F5D67A"),
		progress.WithoutPercentage(),
		progress.WithSpringOptions(20, 1),
	)
	b.EmptyColor = string(trackColor)
	return b
}

// newTempBar builds the temperature slider; its fill sweeps warm to cool
// across the bar so the color itself reads as the kelvin value.
func newTempBar() progress.Model {
	b := progress.New(
		progress.WithWidth(meterWidth),
		progress.WithFillCharacters('█', '█'),
		progress.WithScaledGradient("#FFB86B", "#7FD1FF"),
		progress.WithoutPercentage(),
		progress.WithSpringOptions(20, 1),
	)
	b.EmptyColor = string(trackColor)
	return b
}

func newModel(l light.Light, labels map[string]string, hooks Hooks) model {
	m := model{
		current:       l,
		labels:        labels,
		hooks:         hooks,
		brightnessBar: newBrightnessBar(),
		tempBar:       newTempBar(),
		message:       connectingMessage,
	}
	// Seed with the real state when it is already known so the first paint
	// never flashes wrong values; the refresh loop takes over from there.
	if state, err := l.State(); err == nil {
		m.status = state
		m.message = ""
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

// syncBars points both sliders at the current state so they glide there on
// their internal springs. It takes a pointer receiver ON PURPOSE: with a
// value receiver the SetPercent mutations landed on a throwaway copy, so
// the bars never received their targets and stayed empty.
func (m *model) syncBars() tea.Cmd {
	r := m.current.Ranges()
	var cmds []tea.Cmd
	if p := clamp01(float64(m.status.Brightness) / 100); p != m.brightnessBar.Percent() {
		cmds = append(cmds, m.brightnessBar.SetPercent(p))
	}
	if p := clamp01(float64(m.status.Temp-r.Temp.Min) / float64(rangeSize(r.Temp))); p != m.tempBar.Percent() {
		cmds = append(cmds, m.tempBar.SetPercent(p))
	}
	return tea.Batch(cmds...)
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
	case progress.FrameMsg:
		// Both sliders animate on their internal springs; route frames and
		// keep the returned command alive so the animation runs at 60 fps.
		bm, bcmd := m.brightnessBar.Update(msg)
		m.brightnessBar = bm.(progress.Model)
		tm, tcmd := m.tempBar.Update(msg)
		m.tempBar = tm.(progress.Model)
		return m, tea.Batch(bcmd, tcmd)
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
		slide := m.brightnessBar.SetPercent(clamp01(float64(value) / 100))
		return m, tea.Batch(slide, m.change(func(l light.Light) error { return l.SetBrightness(value) }))
	}
	value := r.Temp.Clamp(m.status.Temp + temp)
	slide := m.tempBar.SetPercent(clamp01(float64(value-r.Temp.Min) / float64(rangeSize(r.Temp))))
	return m, tea.Batch(slide, m.change(func(l light.Light) error { return l.SetTemp(value) }))
}

func identity(l light.Light) string { return l.Driver() + "/" + l.Address() }
