package wiz

import (
	"encoding/json"
	"errors"
	"net"
	"os"
	"sort"
	"time"

	"github.com/lohitcode/study-light/light"
)

func init() { light.Register(Driver{}) }

// Driver connects to WiZ lights over their local UDP protocol.
type Driver struct{}

func (Driver) Name() string { return "wiz" }

// Discover broadcasts getPilot to every network's broadcast address and
// collects replies for up to timeout.
func (Driver) Discover(timeout time.Duration) ([]light.Light, error) {
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4zero, Port: 0})
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	payload, err := json.Marshal(request{Method: "getPilot", Params: map[string]any{}})
	if err != nil {
		return nil, err
	}
	for _, address := range broadcastAddresses() {
		_, _ = conn.WriteToUDP(payload, &net.UDPAddr{IP: address, Port: wizPort})
	}

	deadline := time.Now().Add(timeout)
	found := map[string]struct{}{}
	buffer := make([]byte, 4096)
	var lights []light.Light
	for {
		if err := conn.SetReadDeadline(deadline); err != nil {
			return nil, err
		}
		n, peer, err := conn.ReadFromUDP(buffer)
		if err != nil {
			if errors.Is(err, os.ErrDeadlineExceeded) || isTimeout(err) {
				break
			}
			return nil, err
		}
		var reply response
		if json.Unmarshal(buffer[:n], &reply) == nil && reply.Result != nil {
			host := peer.IP.String()
			if _, seen := found[host]; !seen {
				found[host] = struct{}{}
				lights = append(lights, Light{host: host})
			}
		}
	}
	if len(lights) == 0 {
		return nil, errors.New("no WiZ lights replied")
	}
	sort.Slice(lights, func(i, j int) bool { return lights[i].Address() < lights[j].Address() })
	return lights, nil
}

// Connect verifies the light at address is reachable and returns it.
func (Driver) Connect(address string) (light.Light, error) {
	l := Light{host: address}
	if _, err := l.State(); err != nil {
		return nil, err
	}
	return l, nil
}

func broadcastAddresses() []net.IP {
	addresses := []net.IP{net.IPv4bcast}
	interfaces, _ := net.Interfaces()
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, _ := iface.Addrs()
		for _, address := range addrs {
			ip, network, err := net.ParseCIDR(address.String())
			if err != nil || ip.To4() == nil {
				continue
			}
			ip4, mask := ip.To4(), network.Mask
			broadcast := net.IPv4(ip4[0]|^mask[0], ip4[1]|^mask[1], ip4[2]|^mask[2], ip4[3]|^mask[3])
			addresses = append(addresses, broadcast)
		}
	}
	return addresses
}

func isTimeout(err error) bool {
	netErr, ok := err.(net.Error)
	return ok && netErr.Timeout()
}
