package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/lohitcode/study-light/light"
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
	accent := lipgloss.Color(color)
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFF1C7")).Render("STUDY LIGHT")
	chip := lipgloss.NewStyle().Foreground(lipgloss.Color("#8792BC")).Render("  " + m.current.Label())
	bulb := lipgloss.NewStyle().Foreground(accent).Bold(true).Render("●")
	power := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#F7F8FF")).Render(map[bool]string{true: "ON", false: "OFF"}[state.On])
	if !state.On {
		bulb = lipgloss.NewStyle().Foreground(lipgloss.Color("#596176")).Render("●")
	}
	label := lipgloss.NewStyle().Width(18).Bold(true).Foreground(lipgloss.Color("#E8EBFA"))
	value := lipgloss.NewStyle().Width(8).Align(lipgloss.Right).Foreground(lipgloss.Color("#D2D8F4"))
	rows := []string{
		controlRow(m.cursor == 0, label.Render("Brightness"), meter(brightness-ranges.Brightness.Min, rangeSize(ranges.Brightness), lipgloss.Color("#F5D67A")), value.Render(fmt.Sprintf("%d%%", brightness))),
		controlRow(m.cursor == 1, label.Render("Temperature"), meter(temp-ranges.Temp.Min, rangeSize(ranges.Temp), accent), value.Render(fmt.Sprintf("%d K", temp))),
	}
	status := m.message
	if m.busy {
		status = "Sending local command…"
	}
	if m.mode == modeSearching {
		status = "Searching for lights…"
	}
	if m.err != nil {
		status = "Error: " + m.err.Error()
	}
	statusStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9EA8C8"))
	if m.err != nil {
		statusStyle = statusStyle.Foreground(lipgloss.Color("#FF9B87"))
	}
	section := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#8792BC")).Render("CONTROLS")
	box := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#3B4261")).Padding(1, 2).Width(64).Render(section + "\n\n" + strings.Join(rows, "\n\n"))
	footer := lipgloss.NewStyle().Foreground(lipgloss.Color("#737C9E")).Render("↑↓ select  ←→ adjust  o toggle power  s switch light  q quit")
	current := fmt.Sprintf("%d%%  •  %d K", brightness, temp)
	return "\n  " + title + chip + "\n\n  " + bulb + "  " + power + "     " + current + "\n\n" + box + "\n\n  " + statusStyle.Render(status) + "\n\n  " + footer + "\n"
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
