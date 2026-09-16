package ui

import (
	"errors"
	"math"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/lohitcode/lumen/light"
)

// Tests run without a TTY, where lipgloss would strip all color output;
// force true color so the ANSI-level assertions below see real escapes.
func init() {
	lipgloss.DefaultRenderer().SetColorProfile(termenv.TrueColor)
}

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
		current:       fakeLight{ranges},
		status:        light.State{On: true, Brightness: 70, Temp: 5000},
		message:       "Connected",
		brightnessBar: newBrightnessBar(),
		tempBar:       newTempBar(),
		labels:        map[string]string{},
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

// TestSelectedRowHighlightCoversEveryCell walks the raw ANSI output of a
// selected row and verifies the name chip carries the highlight background
// on every one of its cells while the bar area and value carry none — the
// animated bubbles/progress bar owns its own colors there. Nested style
// resets previously cut the highlight short after the label, leaving an
// asymmetric blob.
func TestSelectedRowHighlightCoversEveryCell(t *testing.T) {
	row := controlRow(true, "Brightness", strings.Repeat("█", meterWidth), "45%")
	cells := cellBackgrounds(row)
	if len(cells) != rowWidth {
		t.Fatalf("selected row renders %d cells, want %d", len(cells), rowWidth)
	}
	const want = "48;2;39;44;67" // selectedBg #272C43
	chip := 2 + labelWidth       // marker + name carry the highlight
	for i, bg := range cells {
		if i < chip && bg != want {
			t.Fatalf("selected row cell %d is missing the highlight background (found %q)", i+1, bg)
		}
		if i >= chip && bg != "" {
			t.Fatalf("selected row cell %d unexpectedly has a background (found %q)", i+1, bg)
		}
	}

	idle := controlRow(false, "Brightness", strings.Repeat("█", meterWidth), "45%")
	for i, bg := range cellBackgrounds(idle) {
		if bg != "" {
			t.Fatalf("unselected row cell %d unexpectedly has background %q", i+1, bg)
		}
	}
}

// TestSyncBarsSetsTargetsFromState is a regression guard for a value-
// receiver bug: syncBars used to mutate a throwaway copy of the model, so
// the sliders never received their targets and stayed empty. With the
// pointer receiver, the targets must land on the model itself.
func TestSyncBarsSetsTargetsFromState(t *testing.T) {
	ranges := light.Ranges{
		Brightness: light.Range{Min: 10, Max: 100},
		Temp:       light.Range{Min: 2700, Max: 6500},
	}
	m := model{
		current:       fakeLight{ranges},
		status:        light.State{On: true, Brightness: 45, Temp: 5300},
		brightnessBar: newBrightnessBar(),
		tempBar:       newTempBar(),
		labels:        map[string]string{},
	}
	m.syncBars() // pointer receiver: must mutate m itself

	if got := m.brightnessBar.Percent(); math.Abs(got-0.45) > 1e-9 {
		t.Fatalf("brightness bar target = %v, want 0.45", got)
	}
	wantTemp := float64(5300-2700) / 3800
	if got := m.tempBar.Percent(); math.Abs(got-wantTemp) > 1e-9 {
		t.Fatalf("temperature bar target = %v, want %v", got, wantTemp)
	}
}

// TestSyncBarsIsIdempotent keeps the 2-second refresh from restarting the
// spring animation when the value has not changed.
func TestSyncBarsIsIdempotent(t *testing.T) {
	ranges := light.Ranges{Brightness: light.Range{Min: 10, Max: 100}, Temp: light.Range{Min: 2700, Max: 6500}}
	m := model{
		current:       fakeLight{ranges},
		status:        light.State{On: true, Brightness: 45, Temp: 5300},
		brightnessBar: newBrightnessBar(),
		tempBar:       newTempBar(),
		labels:        map[string]string{},
	}
	m.syncBars()
	if cmd := m.syncBars(); cmd != nil {
		t.Fatal("second sync with unchanged state should not re-animate the bars")
	}
}

// TestHeaderPrefersAliasOverAddress makes sure a light with a user-set name
// shows the name in the header, and one without falls back to the device
// label (the address).
func TestHeaderPrefersAliasOverAddress(t *testing.T) {
	ranges := light.Ranges{
		Brightness: light.Range{Min: 10, Max: 100},
		Temp:       light.Range{Min: 2700, Max: 6500},
	}
	new := func(labels map[string]string) model {
		return model{
			current:       fakeLight{ranges},
			status:        light.State{On: true, Brightness: 45, Temp: 4900},
			brightnessBar: newBrightnessBar(),
			tempBar:       newTempBar(),
			labels:        labels,
		}
	}

	named := new(map[string]string{"fake/127.0.0.1": "Study Light"})
	if view := named.View(); !strings.Contains(view, "Study Light") || strings.Contains(view, "Fake @ 127.0.0.1") {
		t.Error("header must show the user-set alias when one exists")
	}
	unnamed := new(map[string]string{})
	if view := unnamed.View(); !strings.Contains(view, "Fake @ 127.0.0.1") {
		t.Error("header must fall back to the device label when no alias is set")
	}
}

// cellBackgrounds renders one background color per visible cell by walking
// the string's SGR escape sequences. Glyphs are assumed single-width.
func cellBackgrounds(s string) []string {
	var cells []string
	fg, bg := "", ""
	for i := 0; i < len(s); {
		if s[i] == 0x1b {
			end := strings.IndexByte(s[i:], 'm')
			if end < 0 {
				break
			}
			params := strings.Split(strings.TrimSuffix(s[i:i+end+1], "m"), "\x1b[")
			sgr := ""
			if len(params) == 2 {
				sgr = params[1]
			}
			fg, bg = applySGR(strings.Split(sgr, ";"), fg, bg)
			i += end + 1
			continue
		}
		_, size := utf8.DecodeRuneInString(s[i:])
		cells = append(cells, bg)
		i += size
	}
	return cells
}

func applySGR(params []string, fg, bg string) (string, string) {
	for k := 0; k < len(params); k++ {
		switch p := params[k]; {
		case p == "0" || p == "":
			fg, bg = "", ""
		case p == "39":
			fg = ""
		case p == "49":
			bg = ""
		case (p == "38" || p == "48") && k+4 < len(params) && params[k+1] == "2":
			value := strings.Join(params[k:k+5], ";")
			if p == "38" {
				fg = value
			} else {
				bg = value
			}
			k += 4
		}
	}
	return fg, bg
}

func lineWidths(view string) []int {
	lines := strings.Split(view, "\n")
	widths := make([]int, len(lines))
	for i, line := range lines {
		widths[i] = lipgloss.Width(line)
	}
	return widths
}
