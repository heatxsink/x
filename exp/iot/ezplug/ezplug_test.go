package ezplug

import (
	"os"
	"testing"
	"time"

	"github.com/heatxsink/x/exp/iot"
)

// TODO: Need to make these an env var OR mock?
var (
	brokerAddr   = "tcp://10.0.13.1:1883"
	username     = "dyoru8xjr89zzydf8kmo"
	password     = "dqd2a3b1ikdximbe9y5w"
	clientID     = "ezplug-tests"
	iotClient    *iot.IoT
	ep           *EzPlug
	testEzPlugID = "35ECE9"
	testTopic    string
)

func TestSetup(t *testing.T) {
	if os.Getenv("CI") != "" || os.Getenv("GITHUB_ACTIONS") != "" {
		t.Skip("Skipping test: MQTT broker not available in CI environment")
	}
	iotClient = iot.New(brokerAddr, username, password, clientID, false)
	_, err := iotClient.Connect()
	if err != nil {
		t.Skip("Skipping test: MQTT broker not available -", err)
	}
	ep = New(iotClient)
	testTopic = ep.Topic(testEzPlugID)
}

func TestOnOff(t *testing.T) {
	if ep == nil {
		t.Skip("Skipping test: EzPlug client not initialized")
	}
	d := time.Second * 5
	if err := ep.On(testTopic); err != nil {
		t.Error(err)
	}
	time.Sleep(d)
	if err := ep.Off(testTopic); err != nil {
		t.Error(err)
	}
	time.Sleep(d)
	if err := ep.On(testTopic); err != nil {
		t.Error(err)
	}
}

func TestToggle(t *testing.T) {
	if ep == nil {
		t.Skip("Skipping test: EzPlug client not initialized")
	}
	d := time.Second * 5
	// Turn "On".
	if err := ep.On(testTopic); err != nil {
		t.Error(err)
	}
	time.Sleep(d)
	// Toggle "Off"
	if err := ep.Toggle(testTopic); err != nil {
		t.Error(err)
	}
	time.Sleep(d)
	// Toggle "On"
	if err := ep.Toggle(testTopic); err != nil {
		t.Error(err)
	}
}
