package wled

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/heatxsink/x/exp/iot"
)

// TODO: Need to make these an env var OR mock?
var (
	brokerAddr = "tcp://10.0.15.1:1883"
	username   = "dyoru8xjr89zzydf8kmo"
	password   = "dqd2a3b1ikdximbe9y5w"
	clientID   = "wled-tests"
	iotClient  *iot.IoT
	w          *WLed
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
	w = New(iotClient)
}

func TestOnOff(t *testing.T) {
	if w == nil {
		t.Skip("Skipping test: WLed client not initialized")
	}
	d := time.Second * 5
	if err := w.On(TopicAll); err != nil {
		t.Error(err)
	}
	time.Sleep(d)
	if err := w.Off(TopicAll); err != nil {
		t.Error(err)
	}
	time.Sleep(d)
	if err := w.On(TopicAll); err != nil {
		t.Error(err)
	}
}

func TestToggle(t *testing.T) {
	if w == nil {
		t.Skip("Skipping test: WLed client not initialized")
	}
	d := time.Second * 5
	// Turn "On".
	if err := w.On(TopicAll); err != nil {
		t.Error(err)
	}
	time.Sleep(d)
	// Toggle "Off"
	if err := w.Toggle(TopicAll); err != nil {
		t.Error(err)
	}
	time.Sleep(d)
	// Toggle "On"
	if err := w.Toggle(TopicAll); err != nil {
		t.Error(err)
	}
	time.Sleep(d)
	// Turn "Off"
	if err := w.Off(TopicAll); err != nil {
		t.Error(err)
	}
}

func TestAllBrightness(t *testing.T) {
	if w == nil {
		t.Skip("Skipping test: WLed client not initialized")
	}
	reset(true)
	for i := 10; i <= 255; i = i + 10 {
		if err := w.Brightness(TopicAll, int64(i)); err != nil {
			t.Error(err)
		}
		fmt.Println(TopicAll, " -> ", i)
		time.Sleep(500 * time.Millisecond)
	}
}

func TestAllColor(t *testing.T) {
	if w == nil {
		t.Skip("Skipping test: WLed client not initialized")
	}
	presets := []string{
		"#FC419A",
		"#FF0000",
		"#00FF00",
		"#0000FF",
		"#FBFBF8",
		"#1E90FF",
		"#FE5A1D",
		"#ED008C",
	}
	reset(true)
	for _, c := range presets {
		if err := w.Color(TopicAll, c); err != nil {
			t.Error(err)
		}
		if err := w.Brightness(TopicAll, int64(255)); err != nil {
			t.Error(err)
		}
		time.Sleep(1 * time.Second)
	}
}

func reset(sleep bool) error {
	if err := w.Off(TopicAll); err != nil {
		return err
	}
	time.Sleep(3 * time.Second)
	if err := w.Color(TopicAll, "#FBFBF8"); err != nil {
		return err
	}
	if err := w.Brightness(TopicAll, 30); err != nil {
		return err
	}
	time.Sleep(3 * time.Second)
	if err := w.On(TopicAll); err != nil {
		return err
	}
	if sleep {
		time.Sleep(5 * time.Second)
	}
	return nil
}

func TestAPI(t *testing.T) {
	if w == nil {
		t.Skip("Skipping test: WLed client not initialized")
	}
	if err := w.API(TopicAll, "FX=0"); err != nil {
		t.Error(err)
	}
	if err := w.Color(TopicAll, "#00FF00"); err != nil {
		t.Error(err)
	}
	time.Sleep(10 * time.Second)
	if err := w.API(TopicAll, "FX=91&SX=210&IX=128"); err != nil {
		t.Error(err)
	}
	if err := w.Color(TopicAll, "#0000FF"); err != nil {
		t.Error(err)
	}
	time.Sleep(10 * time.Second)
	if err := w.API(TopicAll, "FX=0"); err != nil {
		t.Error(err)
	}
	if err := w.Color(TopicAll, "#FF0000"); err != nil {
		t.Error(err)
	}
}
