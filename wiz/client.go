// Package wiz speaks the local UDP JSON protocol WiZ lights expose on port
// 38899: getPilot to read state, setPilot to change it, and the registration
// handshake the bulbs expect before accepting writes.
package wiz

import (
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"time"
)

const wizPort = 38899

type request struct {
	Method string         `json:"method"`
	Params map[string]any `json:"params"`
	Env    string         `json:"env,omitempty"`
	ID     int            `json:"id,omitempty"`
}

type Response struct {
	Method string         `json:"method"`
	Env    string         `json:"env"`
	Result map[string]any `json:"result"`
	Error  *RPCError      `json:"error"`
}

type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *RPCError) Error() string { return fmt.Sprintf("%s (%d)", e.Message, e.Code) }

type Device struct {
	Host string
	Info Response
}

// StatusLine renders a one-line summary of the device's last known state.
func (d Device) StatusLine() string {
	if d.Info.Error != nil {
		return "error: " + d.Info.Error.Message
	}
	state, _ := d.Info.Result["state"].(bool)
	return fmt.Sprintf("%s • %v%% • %v K",
		map[bool]string{true: "on", false: "off"}[state],
		d.Info.Result["dimming"], d.Info.Result["temp"])
}

// GetState fetches the light's current power, brightness, and temperature.
func GetState(host string) (Response, error) {
	return Call(host, "getPilot", nil)
}

// SetLight applies params to the light, registering first as the bulbs
// expect. Firmware that rejects setPilot falls back to the legacy setState.
func SetLight(host string, params map[string]any) (Response, error) {
	if err := register(host); err != nil {
		return Response{}, err
	}
	reply, err := Call(host, "setPilot", params)
	if err != nil {
		return Response{}, err
	}
	if reply.Error != nil && reply.Error.Code == -32602 {
		reply, err = Call(host, "setState", params)
	}
	return reply, err
}

// Call sends one UDP request to the light and reads the single response.
func Call(host, method string, params map[string]any) (Response, error) {
	if params == nil {
		params = map[string]any{}
	}
	payload, err := json.Marshal(request{Method: method, Params: params, Env: "pro", ID: 1})
	if err != nil {
		return Response{}, err
	}
	address, err := net.ResolveUDPAddr("udp4", net.JoinHostPort(host, strconv.Itoa(wizPort)))
	if err != nil {
		return Response{}, err
	}
	conn, err := net.DialUDP("udp4", nil, address)
	if err != nil {
		return Response{}, err
	}
	defer conn.Close()
	if _, err := conn.Write(payload); err != nil {
		return Response{}, err
	}
	if err := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		return Response{}, err
	}
	buffer := make([]byte, 4096)
	n, err := conn.Read(buffer)
	if err != nil {
		return Response{}, err
	}
	var reply Response
	if err := json.Unmarshal(buffer[:n], &reply); err != nil {
		return Response{}, err
	}
	return reply, nil
}
