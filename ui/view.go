package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/lohitcode/study-light/light"
)

// Layout geometry. Everything the view renders lives inside these fixed
// widths, so no state change can push neighboring text around.
const (
	contentWidth = 66 // status line and box width, including the indent
	messageLimit = 44 // characters of status message before truncation
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

	label := lipgloss.NewStyle().Width(18).Bold(true).Foreground(lipgloss.Color("#E8EBFA"))
	value := lipgloss.NewStyle().Width(8).Align(lipgloss.Right).Foreground(lipgloss.Color("#D2D8F4"))
	rows := []string{
		controlRow(m.cursor == 0, label.Render("Brightness"), meter(brightness-ranges.Brightness.Min, rangeSize(ranges.Brightness), lipgloss.Color("#F5D67A")), value.Render(fmt.Sprintf("%d%%", brightness))),
		controlRow(m.cursor == 1, label.Render("Temperature"), meter(temp-ranges.Temp.Min, rangeSize(ranges.Temp), lipgloss.Color(color)), value.Render(fmt.Sprintf("%d K", temp))),
	}
	section := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#8792BC")).Render("CONTROLS")
	box := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#3B4261")).Padding(1, 2).Width(64).Render(section + "\n\n" + strings.Join(rows, "\n\n"))
	footer := lipgloss.NewStyle().Foreground(lipgloss.Color("#737C9E")).Render("↑↓ select  ←→ adjust  o toggle power  s switch light  q quit")
	return "\n  " + title + chip + "\n\n  " + bulb + " " + power + current + "\n\n" + box + "\n\n  " + m.statusView() + "\n\n  " + footer + "\n"
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

func (m model) viewPicker() string {
	section := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#8792BC")).Render("SWITCH LIGHT")
	var rows []string
	for i, l := range m.found {
		text := l.Label()
		if identity(l) == identity(m.current) {
			text += "  (current)"
		}
		if i == m.pick {
			rows = append(rows, lipgloss.NewStyle().Foreground(lipgloss.Color("#FFF6D6")).Background(lipgloss.Color("#272C43")).Width(58).Render("› "+text))
			continue
		}
		rows = append(rows, lipgloss.NewStyle().Width(58).Render("  "+text))
	}
	box := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#3B4261")).Padding(1, 2).Width(64).Render(section + "\n\n" + strings.Join(rows, "\n\n"))
	footer := lipgloss.NewStyle().Foreground(lipgloss.Color("#737C9E")).Render("↑↓ choose  enter switch  esc cancel")
	return "\n  " + box + "\n\n  " + footer + "\n"
}

func controlRow(selected bool, label, bar, value string) string {
	row := "  " + label + "  " + bar + "  " + value
	if selected {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#FFF6D6")).Background(lipgloss.Color("#272C43")).Width(58).Render("›" + row)
	}
	return lipgloss.NewStyle().Width(58).Render(" " + row)
}

func meter(value, max int, color lipgloss.Color) string {
	const width = 25
	filled := value * width / max
	if filled < 0 {
		filled = 0
	}
	if filled > width {
		filled = width
	}
	return lipgloss.NewStyle().Foreground(color).Render(strings.Repeat("━", filled)) + lipgloss.NewStyle().Foreground(lipgloss.Color("#343B56")).Render(strings.Repeat("━", width-filled))
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
