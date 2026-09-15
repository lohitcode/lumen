package ui

import (
	"errors"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/lohitcode/study-light/light"
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

// TestSelectedRowHighlightCoversEveryCell walks the raw ANSI output of a
// selected row and verifies every visible cell carries the highlight
// background — nested style resets previously cut the highlight short after
// the label, leaving an asymmetric blob.
func TestSelectedRowHighlightCoversEveryCell(t *testing.T) {
	row := controlRow(true, "Brightness", "45%", 35, 90, lipgloss.Color("#F5D67A"))
	cells := cellBackgrounds(row)
	if len(cells) != rowWidth {
		t.Fatalf("selected row renders %d cells, want %d", len(cells), rowWidth)
	}
	const want = "48;2;39;44;67" // selectedBg #272C43
	for i, bg := range cells {
		if bg != want {
			t.Fatalf("selected row cell %d is missing the highlight background (found %q)", i+1, bg)
		}
	}

	idle := controlRow(false, "Brightness", "45%", 35, 90, lipgloss.Color("#F5D67A"))
	for i, bg := range cellBackgrounds(idle) {
		if bg != "" {
			t.Fatalf("unselected row cell %d unexpectedly has background %q", i+1, bg)
		}
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
