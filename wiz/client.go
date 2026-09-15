// Package wiz implements the local UDP JSON protocol WiZ lights expose on
// port 38899: getPilot to read state, setPilot to change it, and the
// registration handshake the bulbs expect before accepting writes.
package wiz

import (
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

const wizPort = 38899

type request struct {
	Method string         `json:"method"`
	Params map[string]any `json:"params"`
	Env    string         `json:"env,omitempty"`
	ID     int            `json:"id,omitempty"`
}

type response struct {
	Method string         `json:"method"`
	Env    string         `json:"env"`
	Result map[string]any `json:"result"`
	Error  *rpcError      `json:"error"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *rpcError) Error() string { return fmt.Sprintf("%s (%d)", e.Message, e.Code) }

// getState fetches the light's current power, brightness, and temperature.
func getState(host string) (response, error) {
	return call(host, "getPilot", nil)
}

// deviceName fetches the friendly name the user set for the bulb in the
// WiZ app, or an empty string when it can't be read.
func deviceName(host string) string {
	reply, err := call(host, "getSystemConfig", nil)
	if err != nil || reply.Error != nil {
		return ""
	}
	name, _ := reply.Result["friendlyName"].(string)
	return strings.TrimSpace(name)
}

// setLight applies params to the light, registering first as the bulbs
// expect. Firmware that rejects setPilot falls back to the legacy setState.
func setLight(host string, params map[string]any) (response, error) {
	if err := register(host); err != nil {
		return response{}, err
	}
	reply, err := call(host, "setPilot", params)
	if err != nil {
		return response{}, err
	}
	if reply.Error != nil && reply.Error.Code == -32602 {
		reply, err = call(host, "setState", params)
	}
	return reply, err
}

// call sends one UDP request to the light and reads the single response.
func call(host, method string, params map[string]any) (response, error) {
	if params == nil {
		params = map[string]any{}
	}
	payload, err := json.Marshal(request{Method: method, Params: params, Env: "pro", ID: 1})
	if err != nil {
		return response{}, err
	}
	address, err := net.ResolveUDPAddr("udp4", net.JoinHostPort(host, strconv.Itoa(wizPort)))
	if err != nil {
		return response{}, err
	}
	conn, err := net.DialUDP("udp4", nil, address)
	if err != nil {
		return response{}, err
	}
	defer conn.Close()
	if _, err := conn.Write(payload); err != nil {
		return response{}, err
	}
	if err := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		return response{}, err
	}
	buffer := make([]byte, 4096)
	n, err := conn.Read(buffer)
	if err != nil {
		return response{}, err
	}
	var reply response
	if err := json.Unmarshal(buffer[:n], &reply); err != nil {
		return response{}, err
	}
	return reply, nil
}
