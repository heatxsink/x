package ezplug

import (
	"testing"
	"time"

	"github.com/heatxsink/x/iot"
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
	iotClient = iot.New(brokerAddr, username, password, clientID, false)
	_, err := iotClient.Connect()
	if err != nil {
		t.Error(err)
	}
	ep = New(iotClient)
	testTopic = ep.Topic(testEzPlugID)
}

func TestOnOff(t *testing.T) {
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
