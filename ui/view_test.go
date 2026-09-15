package ui

import (
	"errors"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"github.com/lohitcode/study-light/light"
)

type fakeLight struct{ ranges light.Ranges }

func (f fakeLight) Driver() string  { return "fake" }
func (f fakeLight) Address() string { return "127.0.0.1" }
func (f fakeLight) Label() string   { return "Fake @ 127.0.0.1" }
func (f fakeLight) Ranges() light.Ranges {
	return f.ranges
}
func (fakeLight) State() (light.State, error) {
	return light.State{On: true, Brightness: 70, Temp: 5000}, nil
}
func (fakeLight) SetPower(bool) error     { return nil }
func (fakeLight) SetBrightness(int) error { return nil }
func (fakeLight) SetTemp(int) error       { return nil }

// TestViewLineWidthsAreStable guards against layout jitter: every line of
// the dashboard must keep the same rendered width across state changes, so
// nothing shifts or blinks between repaints.
func TestViewLineWidthsAreStable(t *testing.T) {
	ranges := light.Ranges{
		Brightness: light.Range{Min: 10, Max: 100},
		Temp:       light.Range{Min: 2700, Max: 6500},
	}
	base := model{
		current: fakeLight{ranges},
		status:  light.State{On: true, Brightness: 70, Temp: 5000},
		message: "Connected",
	}

	states := map[string]model{
		"live":           base,
		"busy":           withModel(base, func(m *model) { m.busy = true; m.frame = 3 }),
		"error":          withModel(base, func(m *model) { m.err = errors.New("read udp: i/o timeout") }),
		"long error":     withModel(base, func(m *model) { m.err = errors.New("very long failure: " + strings.Repeat("x", 120)) }),
		"long message":   withModel(base, func(m *model) { m.message = strings.Repeat("y", 120) }),
		"powered off":    withModel(base, func(m *model) { m.status.On = false }),
		"max brightness": withModel(base, func(m *model) { m.status.Brightness = 100 }),
		"min brightness": withModel(base, func(m *model) { m.status.Brightness = 10 }),
		"warm temp":      withModel(base, func(m *model) { m.status.Temp = 2700 }),
		"row 2":          withModel(base, func(m *model) { m.cursor = 1 }),
		"connecting":     withModel(base, func(m *model) { m.message = connectingMessage; m.status = light.State{} }),
	}

	reference := lineWidths(base.View())
	for name, m := range states {
		lines := lineWidths(m.View())
		if len(lines) != len(reference) {
			t.Errorf("%s: line count changed: got %d, want %d", name, len(lines), len(reference))
			continue
		}
		for i := range reference {
			if lines[i] != reference[i] {
				t.Errorf("%s: line %d width changed: got %d, want %d", name, i+1, lines[i], reference[i])
			}
		}
	}
}

func withModel(m model, mutate func(*model)) model {
	mutate(&m)
	return m
}

func lineWidths(view string) []int {
	lines := strings.Split(view, "\n")
	widths := make([]int, len(lines))
	for i, line := range lines {
		widths[i] = lipgloss.Width(line)
	}
	return widths
}
