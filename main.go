// Command lumen is a terminal UI for controlling smart lights on the
// local network without going through any cloud. It ships with a WiZ driver;
// other lights can be added by implementing the light.Light interface.
package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/lohitcode/lumen/config"
	"github.com/lohitcode/lumen/light"
	"github.com/lohitcode/lumen/ui"

	// Built-in drivers. Add new ones here.
	_ "github.com/lohitcode/lumen/wiz"
)

const discoverTimeout = 2 * time.Second

func main() {
	host := flag.String("host", "", "light address; discovers lights when omitted")
	driverName := flag.String("driver", "wiz", "driver used with --host")
	name := flag.String("name", "", "display name for the light; stored locally and on WiZ bulbs that accept names")
	flag.Parse()

	selected, err := selectLight(*host, *driverName)
	if err != nil {
		fatal(err)
	}

	saved, _ := config.LoadLast()
	label := *name
	if label == "" && saved.Driver == selected.Driver() && saved.Address == selected.Address() {
		label = saved.Name
	}
	if label != "" {
		if setter, ok := selected.(interface{ SetName(string) error }); ok {
			if err := setter.SetName(label); err != nil {
				// Firmware that rejects local names is fine; the label still
				// applies in the UI and persists in the config file.
				fmt.Fprintln(os.Stderr, "lumen: device kept its current name:", err)
			}
		}
		selected = namedLight{Light: selected, label: label}
	}

	if err := config.SaveLast(config.LastLight{
		Driver:  selected.Driver(),
		Address: selected.Address(),
		Name:    label,
	}); err != nil {
		fmt.Fprintln(os.Stderr, "lumen: could not save last light:", err)
	}
	if err := ui.Run(selected, func(l light.Light) {
		_ = config.SaveLast(config.LastLight{
			Driver:  l.Driver(),
			Address: l.Address(),
			Name:    overrideLabel(l),
		})
	}); err != nil {
		fatal(err)
	}
}

// namedLight overrides only the display label of a light and delegates
// everything else.
type namedLight struct {
	light.Light
	label string
}

func (n namedLight) Label() string { return n.label }

// overrideLabel reports a light's user-chosen label, or "" when the light
// only has its device-provided label.
func overrideLabel(l light.Light) string {
	if n, ok := l.(namedLight); ok {
		return n.label
	}
	return ""
}

// selectLight picks the light to open: an explicit address, the light saved
// from the previous session, or discovery.
func selectLight(host, driverName string) (light.Light, error) {
	if host != "" {
		d, err := light.Lookup(driverName)
		if err != nil {
			return nil, err
		}
		return d.Connect(host)
	}

	if last, ok := config.LoadLast(); ok {
		if d, err := light.Lookup(last.Driver); err == nil {
			if l, err := d.Connect(last.Address); err == nil {
				return l, nil
			}
		}
		fmt.Fprintln(os.Stderr, "lumen: last light", last.Address, "is not reachable, discovering…")
	}

	found, err := light.DiscoverAll(discoverTimeout)
	if err != nil {
		return nil, err
	}
	if len(found) == 1 {
		return found[0], nil
	}

	for i, l := range found {
		fmt.Printf("%d. %s\n", i+1, l.Label())
	}
	choice, err := askNumber("Choose a light", 1, len(found))
	if err != nil {
		return nil, err
	}
	return found[choice-1], nil
}

func askNumber(label string, min, max int) (int, error) {
	fmt.Printf("%s: ", label)
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return 0, err
	}
	value, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil || value < min || value > max {
		return 0, errors.New("out of range")
	}
	return value, nil
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "lumen:", err)
	os.Exit(1)
}
