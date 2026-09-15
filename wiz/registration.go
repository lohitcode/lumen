package wiz

import (
	"errors"
	"net"
	"strconv"
	"strings"
)

// register performs the handshake the bulbs expect before accepting writes
// from a new controller on the network.
func register(host string) error {
	ip, mac, err := localIdentity(host)
	if err != nil {
		return err
	}
	reply, err := Call(host, "registration", map[string]any{
		"phoneIp":  ip,
		"phoneMac": mac,
		"register": true,
	})
	if err != nil {
		return err
	}
	if reply.Error != nil {
		return reply.Error
	}
	return nil
}

// localIdentity returns this machine's IP on the route to the light plus the
// matching interface's MAC address, which the registration call asks for.
func localIdentity(host string) (string, string, error) {
	conn, err := net.Dial("udp4", net.JoinHostPort(host, strconv.Itoa(wizPort)))
	if err != nil {
		return "", "", err
	}
	local := conn.LocalAddr().(*net.UDPAddr).IP.String()
	_ = conn.Close()
	interfaces, err := net.Interfaces()
	if err != nil {
		return "", "", err
	}
	for _, iface := range interfaces {
		addresses, _ := iface.Addrs()
		for _, address := range addresses {
			ip, _, _ := net.ParseCIDR(address.String())
			if ip != nil && ip.String() == local && len(iface.HardwareAddr) > 0 {
				return local, strings.ReplaceAll(iface.HardwareAddr.String(), ":", ""), nil
			}
		}
	}
	return "", "", errors.New("could not determine this machine's network address")
}
