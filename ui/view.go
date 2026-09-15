package ui

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m model) View() string {
	if m.quitting {
		return "\n  Study Light disconnected.\n\n"
	}
	state := boolValue(m.status.Result, "state")
	brightness := intValue(m.status.Result, "dimming")
	temp := intValue(m.status.Result, "temp")
	if brightness == 0 {
		brightness = 100
	}
	if temp == 0 {
		temp = 6500
	}
	color := temperatureColor(temp)
	accent := lipgloss.Color(color)
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFF1C7")).Render("STUDY LIGHT")
	chip := lipgloss.NewStyle().Foreground(lipgloss.Color("#8792BC")).Render("  WiZ · " + m.device.Host)
	bulb := lipgloss.NewStyle().Foreground(accent).Bold(true).Render("●")
	power := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#F7F8FF")).Render(map[bool]string{true: "ON", false: "OFF"}[state])
	if !state {
		bulb = lipgloss.NewStyle().Foreground(lipgloss.Color("#596176")).Render("●")
	}
	label := lipgloss.NewStyle().Width(18).Bold(true).Foreground(lipgloss.Color("#E8EBFA"))
	value := lipgloss.NewStyle().Width(8).Align(lipgloss.Right).Foreground(lipgloss.Color("#D2D8F4"))
	rows := []string{
		controlRow(m.cursor == 0, label.Render("Brightness"), meter(brightness-10, 90, lipgloss.Color("#F5D67A")), value.Render(fmt.Sprintf("%d%%", brightness))),
		controlRow(m.cursor == 1, label.Render("Temperature"), meter(temp-2700, 3800, accent), value.Render(fmt.Sprintf("%d K", temp))),
	}
	status := m.message
	if m.busy {
		status = "Sending local command…"
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
	footer := lipgloss.NewStyle().Foreground(lipgloss.Color("#737C9E")).Render("↑↓ select  ←→ adjust  o toggle power  q quit")
	current := fmt.Sprintf("%d%%  •  %d K", brightness, temp)
	return "\n  " + title + chip + "\n\n  " + bulb + "  " + power + "     " + current + "\n\n" + box + "\n\n  " + statusStyle.Render(status) + "\n\n  " + footer + "\n"
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

// temperatureColor maps kelvin values from warm 2700K to cool 6500K.
func temperatureColor(temp int) string {
	p := float64(temp-2700) / 3800
	r := int(255 - p*110)
	g := int(172 + p*46)
	b := int(90 + p*160)
	return fmt.Sprintf("#%02X%02X%02X", r, g, b)
}

func intValue(values map[string]any, key string) int {
	switch v := values[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case json.Number:
		n, _ := v.Int64()
		return int(n)
	}
	return 0
}

func boolValue(values map[string]any, key string) bool { v, _ := values[key].(bool); return v }
