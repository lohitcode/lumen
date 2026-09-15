// Package light defines the small interface every supported light and
// protocol implements. Study Light's UI only talks to these interfaces, so
// adding support for a new brand or protocol means writing one driver — the
// terminal UI, discovery picking, switching, and persistence all keep working.
package light

import (
	"errors"
	"sort"
	"time"
)

// State is a snapshot of a light's current settings.
type State struct {
	On         bool
	Brightness int // percent
	Temp       int // white temperature in kelvin
}

// Range bounds a single value; drivers define the real limits of their
// hardware and the UI clamps against them.
type Range struct{ Min, Max int }

// Clamp returns value constrained to the range.
func (r Range) Clamp(value int) int {
	if value < r.Min {
		return r.Min
	}
	if value > r.Max {
		return r.Max
	}
	return value
}

// Ranges holds the hardware limits of a light.
type Ranges struct {
	Brightness Range
	Temp       Range
}

// Light is a single controllable lamp. Implement this plus a Driver to
// support a new kind of light.
type Light interface {
	// Driver returns the name of the driver this light came from.
	Driver() string
	// Address returns the light's reachable address (e.g. an IP or host).
	Address() string
	// Label returns a human-readable name for pickers and the status bar.
	Label() string
	// Ranges returns the hardware limits used for clamping and display.
	Ranges() Ranges
	// State reads the light's current state.
	State() (State, error)
	// SetPower turns the light on or off.
	SetPower(on bool) error
	// SetBrightness sets brightness in percent; drivers clamp to hardware.
	SetBrightness(percent int) error
	// SetTemp sets the white temperature in kelvin; drivers clamp to hardware.
	SetTemp(kelvin int) error
}

// Driver connects to one brand or protocol of lights.
type Driver interface {
	// Name identifies the driver in saved state and the --driver flag.
	Name() string
	// Discover finds lights on the local network, waiting up to timeout.
	Discover(timeout time.Duration) ([]Light, error)
	// Connect re-attaches to a previously seen light by address, verifying
	// that it answers.
	Connect(address string) (Light, error)
}

var drivers = map[string]Driver{}

// Register makes a driver available to discovery, switching, and reconnects.
// Call it from your driver package's init function.
func Register(d Driver) { drivers[d.Name()] = d }

// Lookup returns the registered driver with the given name.
func Lookup(name string) (Driver, error) {
	d, ok := drivers[name]
	if !ok {
		return nil, errors.New("unknown driver " + name)
	}
	return d, nil
}

// Drivers returns all registered drivers sorted by name.
func Drivers() []Driver {
	names := make([]string, 0, len(drivers))
	for name := range drivers {
		names = append(names, name)
	}
	sort.Strings(names)
	result := make([]Driver, 0, len(names))
	for _, name := range names {
		result = append(result, drivers[name])
	}
	return result
}

// DiscoverAll asks every registered driver to find lights. Drivers that find
// nothing are skipped; an error is returned only when no driver finds any
// light at all.
func DiscoverAll(timeout time.Duration) ([]Light, error) {
	var all []Light
	for _, d := range Drivers() {
		found, err := d.Discover(timeout)
		if err != nil {
			continue
		}
		all = append(all, found...)
	}
	if len(all) == 0 {
		return nil, errors.New("no lights found; confirm your lights are on the same network and reachable")
	}
	return all, nil
}
