// Command study-light is a terminal UI for controlling smart lights on the
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

	"github.com/lohitcode/study-light/config"
	"github.com/lohitcode/study-light/light"
	"github.com/lohitcode/study-light/ui"

	// Built-in drivers. Add new ones here.
	_ "github.com/lohitcode/study-light/wiz"
)

const discoverTimeout = 2 * time.Second

func main() {
	host := flag.String("host", "", "light address; discovers lights when omitted")
	driverName := flag.String("driver", "wiz", "driver used with --host")
	flag.Parse()

	selected, err := selectLight(*host, *driverName)
	if err != nil {
		fatal(err)
	}
	if err := config.SaveLast(selected.Driver(), selected.Address()); err != nil {
		fmt.Fprintln(os.Stderr, "study-light: could not save last light:", err)
	}
	if err := ui.Run(selected, func(l light.Light) {
		_ = config.SaveLast(l.Driver(), l.Address())
	}); err != nil {
		fatal(err)
	}
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
		fmt.Fprintln(os.Stderr, "study-light: last light", last.Address, "is not reachable, discovering…")
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
	fmt.Fprintln(os.Stderr, "study-light:", err)
	os.Exit(1)
}
