// Command study-light is a terminal UI for controlling WiZ lights on the
// local network without going through the WiZ cloud.
package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/lohitcode/study-light/ui"
	"github.com/lohitcode/study-light/wiz"
)

func main() {
	host := flag.String("host", "", "WiZ light IP address; discovers lights when omitted")
	flag.Parse()

	selected, err := selectDevice(*host)
	if err != nil {
		fatal(err)
	}
	if err := ui.Run(selected); err != nil {
		fatal(err)
	}
}

func selectDevice(host string) (wiz.Device, error) {
	if host != "" {
		status, err := wiz.GetState(host)
		return wiz.Device{Host: host, Info: status}, err
	}

	devices, err := wiz.Discover()
	if err != nil {
		return wiz.Device{}, err
	}
	if len(devices) == 1 {
		return devices[0], nil
	}

	for i, d := range devices {
		fmt.Printf("%d. %s  %s\n", i+1, d.Host, d.StatusLine())
	}
	choice, err := askNumber("Choose a light", 1, len(devices))
	if err != nil {
		return wiz.Device{}, err
	}
	return devices[choice-1], nil
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
