package wiz

import (
	"encoding/json"
	"errors"
	"net"
	"os"
	"sort"
	"time"
)

// Discover finds WiZ lights on the local network by broadcasting getPilot to
// every interface's broadcast address and collecting replies for two seconds.
func Discover() ([]Device, error) {
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

	deadline := time.Now().Add(2 * time.Second)
	found := map[string]Device{}
	buffer := make([]byte, 4096)
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
		var reply Response
		if json.Unmarshal(buffer[:n], &reply) == nil && reply.Result != nil {
			found[peer.IP.String()] = Device{Host: peer.IP.String(), Info: reply}
		}
	}

	if len(found) == 0 {
		return nil, errors.New("no WiZ lights replied; confirm this machine and the light are on the same network and local communication is enabled in the WiZ app")
	}
	devices := make([]Device, 0, len(found))
	for _, d := range found {
		devices = append(devices, d)
	}
	sort.Slice(devices, func(i, j int) bool { return devices[i].Host < devices[j].Host })
	return devices, nil
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
