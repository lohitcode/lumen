package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/lohitcode/study-light/light"
)

// Layout geometry. Everything the view renders lives inside these fixed
// widths, so no state change can shift neighboring text.
const (
	contentWidth = 66 // status line width, including the indent
	boxWidth     = 64 // controls box outer width
	boxPaddingX  = 2  // box horizontal padding
	rowWidth     = boxWidth - 2*boxPaddingX
	labelWidth   = 14
	meterWidth   = 25
	valueWidth   = 13
	gapWidth     = 3
	messageLimit = 44 // characters of status message before truncation
)

// Selection colors. Every fragment of a selected row carries the background
// itself — nesting styled strings inside one background-styled render does
// not work: the inner styles' reset sequences would cut the highlight short.
var (
	selectedBg = lipgloss.Color("#272C43")
	selectedFg = lipgloss.Color("#FFF6D6")
	trackColor = lipgloss.Color("#343B56")
)

func (m model) View() string {
	if m.quitting {
		return "\n  Study Light disconnected.\n\n"
	}
	if m.mode == modePicking {
		return m.viewPicker()
	}

	state := m.status
	brightness, temp := state.Brightness, state.Temp
	if brightness == 0 {
		brightness = 100
	}
	if temp == 0 {
		temp = 6500
	}
	ranges := m.current.Ranges()
	color := temperatureColor(temp, ranges.Temp)

	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFF1C7")).Render("STUDY LIGHT")
	chip := lipgloss.NewStyle().Foreground(lipgloss.Color("#8792BC")).Render("  " + m.current.Label())

	// Fixed-width power badge: ON and OFF occupy the same space, so the
	// values after it never shift when power toggles.
	bulbColor := lipgloss.Color(color)
	powerText, powerColor := "ON", "#7FD962"
	if !state.On {
		bulbColor = lipgloss.Color("#596176")
		powerText, powerColor = "OFF", "#596176"
	}
	bulb := lipgloss.NewStyle().Foreground(bulbColor).Bold(true).Render("●")
	power := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(powerColor)).Width(4).Render(powerText)
	current := lipgloss.NewStyle().Foreground(lipgloss.Color("#D2D8F4")).Width(14).Render(
		fmt.Sprintf("%d%% • %d K", brightness, temp))

	rows := []string{
		controlRow(m.cursor == 0, "Brightness", fmt.Sprintf("%d%%", brightness),
			brightness-ranges.Brightness.Min, rangeSize(ranges.Brightness), lipgloss.Color("#F5D67A")),
		controlRow(m.cursor == 1, "Temperature", fmt.Sprintf("%d K", temp),
			temp-ranges.Temp.Min, rangeSize(ranges.Temp), lipgloss.Color(color)),
	}
	section := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#8792BC")).Render("CONTROLS")
	box := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#3B4261")).
		Padding(1, boxPaddingX).Width(boxWidth).Render(section + "\n\n" + strings.Join(rows, "\n\n"))
	return "\n  " + title + chip + "\n\n  " + bulb + " " + power + current + "\n\n" + box + "\n\n  " + m.statusView() + "\n\n  " + m.footerView() + "\n"
}

// controlRow renders one slider row as fixed-width segments. When selected,
// every segment (including the gaps) shares the highlight background, so the
// highlight is one clean symmetric bar spanning the whole row.
func controlRow(selected bool, name, valueText string, value, maxValue int, barColor lipgloss.Color) string {
	tint := func(s lipgloss.Style) lipgloss.Style {
		if selected {
			return s.Background(selectedBg)
		}
		return s
	}
	prefix := " "
	if selected {
		prefix = "›"
	}
	segments := []string{
		tint(lipgloss.NewStyle().Bold(true).Foreground(selectedFg)).Width(2).Render(prefix),
		tint(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#E8EBFA"))).Width(labelWidth).Render(name),
		tint(lipgloss.NewStyle()).Render(strings.Repeat(" ", gapWidth)),
		tint(lipgloss.NewStyle()).Render(slider(value, maxValue, barColor, selected)),
		tint(lipgloss.NewStyle()).Render(strings.Repeat(" ", gapWidth)),
		tint(lipgloss.NewStyle().Foreground(lipgloss.Color("#D2D8F4"))).Width(valueWidth).
			Align(lipgloss.Right).Render(valueText),
	}
	return strings.Join(segments, "")
}

// slider renders a meter as segmented terminal blocks, always exactly
// meterWidth cells wide. The track uses the full block glyph too (in a dim
// color) so the whole bar is one solid band — partial-shade glyphs would
// break up the highlight with a ragged bottom edge.
func slider(value, maxValue int, color lipgloss.Color, selected bool) string {
	if maxValue <= 0 {
		maxValue = 1
	}
	pos := value * meterWidth / maxValue
	if pos < 0 {
		pos = 0
	}
	if pos > meterWidth {
		pos = meterWidth
	}
	filled := lipgloss.NewStyle().Foreground(color)
	track := lipgloss.NewStyle().Foreground(trackColor)
	if selected {
		filled = filled.Background(selectedBg)
		track = track.Background(selectedBg)
	}
	return filled.Render(strings.Repeat("█", pos)) +
		track.Render(strings.Repeat("█", meterWidth-pos))
}

// statusView renders the bottom status line as fixed-width segments: an
// indicator icon, a state word, and the current message. The line keeps the
// same geometry in every state, so it never jumps or flickers between
// repaints.
func (m model) statusView() string {
	var icon, color, word string
	switch {
	case m.mode == modeSearching:
		icon, color, word = spinnerFrames[m.frame], "#F5D67A", "scan"
	case m.busy:
		icon, color, word = spinnerFrames[m.frame], "#F5D67A", "sync"
	case m.err != nil:
		icon, color, word = "✕", "#FF9B87", "error"
	default:
		icon, color, word = "●", "#7FD962", "live"
	}
	text := m.message
	textColor := "#9EA8C8"
	if m.err != nil {
		text = "Error: " + m.err.Error()
		textColor = "#FF9B87"
	}
	indicator := lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Width(7).Render(icon + " " + word)
	return lipgloss.NewStyle().Width(contentWidth).Render(
		indicator + lipgloss.NewStyle().Foreground(lipgloss.Color(textColor)).Render(truncate(text, messageLimit)))
}

// footerView renders the key hints with keycap-style colored keys.
func (m model) footerView() string {
	key := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#F5D67A"))
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("#737C9E"))
	return key.Render("↑↓") + dim.Render(" select  ") +
		key.Render("←→") + dim.Render(" adjust  ") +
		key.Render("o") + dim.Render(" power  ") +
		key.Render("s") + dim.Render(" lights  ") +
		key.Render("q") + dim.Render(" quit")
}

func (m model) viewPicker() string {
	section := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#8792BC")).Render("SWITCH LIGHT")
	var rows []string
	for i, l := range m.found {
		text := l.Label()
		if identity(l) == identity(m.current) {
			text += "  (current)"
		}
		if i == m.pick {
			rows = append(rows, lipgloss.NewStyle().Foreground(selectedFg).Background(selectedBg).
				Width(rowWidth).Render("› "+text))
			continue
		}
		rows = append(rows, lipgloss.NewStyle().Width(rowWidth).Render("  "+text))
	}
	box := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#3B4261")).
		Padding(1, boxPaddingX).Width(boxWidth).Render(section + "\n\n" + strings.Join(rows, "\n\n"))
	return "\n  " + box + "\n\n  " + m.footerView() + "\n"
}

func rangeSize(r light.Range) int {
	size := r.Max - r.Min
	if size <= 0 {
		return 1
	}
	return size
}

// temperatureColor maps kelvin values from warm to cool across the light's
// supported range.
func temperatureColor(temp int, r light.Range) string {
	p := float64(temp-r.Min) / float64(rangeSize(r))
	if p < 0 {
		p = 0
	}
	if p > 1 {
		p = 1
	}
	red := int(255 - p*110)
	green := int(172 + p*46)
	blue := int(90 + p*160)
	return fmt.Sprintf("#%02X%02X%02X", red, green, blue)
}

// truncate shortens text to limit runes so the status line stays single-line
// no matter how long a message or error is.
func truncate(text string, limit int) string {
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	if limit < 1 {
		return ""
	}
	return string(runes[:limit-1]) + "…"
}
