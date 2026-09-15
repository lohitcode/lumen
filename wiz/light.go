package wiz

import (
	"encoding/json"

	"github.com/lohitcode/study-light/light"
)

// Light is a single WiZ bulb on the local network.
type Light struct {
	host string
}

var wizRanges = light.Ranges{
	Brightness: light.Range{Min: 10, Max: 100},
	Temp:       light.Range{Min: 2700, Max: 6500},
}

func (l Light) Driver() string  { return "wiz" }
func (l Light) Address() string { return l.host }
func (l Light) Label() string   { return "WiZ @ " + l.host }
func (l Light) Ranges() light.Ranges {
	return wizRanges
}

// State reads the bulb's current power, brightness, and temperature.
func (l Light) State() (light.State, error) {
	reply, err := getState(l.host)
	if err != nil {
		return light.State{}, err
	}
	if reply.Error != nil {
		return light.State{}, reply.Error
	}
	return light.State{
		On:         boolValue(reply.Result, "state"),
		Brightness: intValue(reply.Result, "dimming"),
		Temp:       intValue(reply.Result, "temp"),
	}, nil
}

// SetPower turns the bulb on or off.
func (l Light) SetPower(on bool) error {
	return l.apply(map[string]any{"state": on})
}

// SetBrightness sets brightness in percent, clamped to the bulb's range.
func (l Light) SetBrightness(percent int) error {
	return l.apply(map[string]any{"dimming": wizRanges.Brightness.Clamp(percent)})
}

// SetTemp sets the white temperature in kelvin, clamped to the bulb's range.
func (l Light) SetTemp(kelvin int) error {
	return l.apply(map[string]any{"temp": wizRanges.Temp.Clamp(kelvin)})
}

// apply sends one write and surfaces transport errors and bulb rejections.
func (l Light) apply(params map[string]any) error {
	reply, err := setLight(l.host, params)
	if err != nil {
		return err
	}
	if reply.Error != nil {
		return reply.Error
	}
	return nil
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

func boolValue(values map[string]any, key string) bool {
	v, _ := values[key].(bool)
	return v
}

var _ light.Light = Light{}
