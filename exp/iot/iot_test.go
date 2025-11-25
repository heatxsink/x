package iot

import (
	"os"
	"testing"
)

var (
	brokerAddr = "tcp://10.0.13.1:1883"
	username   = "dyoru8xjr89zzydf8kmo"
	password   = "dqd2a3b1ikdximbe9y5w"
	clientID   = "iot-tests"
	iotClient  *IoT
)

func TestConnect(t *testing.T) {
	if os.Getenv("CI") != "" || os.Getenv("GITHUB_ACTIONS") != "" {
		t.Skip("Skipping test: MQTT broker not available in CI environment")
	}
	iotClient = New(brokerAddr, username, password, clientID, false)
	_, err := iotClient.Connect()
	if err != nil {
		t.Skip("Skipping test: MQTT broker not available -", err)
	}
}
