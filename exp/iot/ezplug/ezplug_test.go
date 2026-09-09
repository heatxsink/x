package ezplug

import (
	"testing"
	"time"

	"github.com/heatxsink/x/exp/iot"
	"github.com/heatxsink/x/exp/iot/internal/mqtttest"
)

const (
	testUsername = "test-user"
	testPassword = "test-password"
	testClientID = "ezplug-tests"
	testEzPlugID = "35ECE9"
)

// newEzPlug returns an EzPlug wired to an embedded broker, plus that broker so
// the caller can assert on what was published.
func newEzPlug(t *testing.T) (*EzPlug, *mqtttest.Broker) {
	t.Helper()
	b := mqtttest.New(t, mqtttest.WithCredentials(testUsername, testPassword))
	c := iot.New(b.Addr(), testUsername, testPassword, testClientID, true)
	if _, err := c.Connect(); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	return New(c), b
}

func TestTopic(t *testing.T) {
	ep := New(nil)

	if got, want := ep.Topic(testEzPlugID), "cmnd/EZPlug_35ECE9/Power"; got != want {
		t.Errorf("Topic(%q) = %q, want %q", testEzPlugID, got, want)
	}
}

func TestTopicEmptyID(t *testing.T) {
	ep := New(nil)

	if got, want := ep.Topic(""), "cmnd/EZPlug_/Power"; got != want {
		t.Errorf("Topic(\"\") = %q, want %q", got, want)
	}
}

func TestOn(t *testing.T) {
	ep, b := newEzPlug(t)
	topic := ep.Topic(testEzPlugID)

	if err := ep.On(topic); err != nil {
		t.Fatalf("On: %v", err)
	}

	msgs := b.WaitForTopic(t, topic, 1, 5*time.Second)
	if msgs[0].Payload != On {
		t.Errorf("payload = %q, want %q", msgs[0].Payload, On)
	}
}

func TestOff(t *testing.T) {
	ep, b := newEzPlug(t)
	topic := ep.Topic(testEzPlugID)

	if err := ep.Off(topic); err != nil {
		t.Fatalf("Off: %v", err)
	}

	msgs := b.WaitForTopic(t, topic, 1, 5*time.Second)
	if msgs[0].Payload != Off {
		t.Errorf("payload = %q, want %q", msgs[0].Payload, Off)
	}
}

func TestToggle(t *testing.T) {
	ep, b := newEzPlug(t)
	topic := ep.Topic(testEzPlugID)

	if err := ep.Toggle(topic); err != nil {
		t.Fatalf("Toggle: %v", err)
	}

	msgs := b.WaitForTopic(t, topic, 1, 5*time.Second)
	if msgs[0].Payload != Toggle {
		t.Errorf("payload = %q, want %q", msgs[0].Payload, Toggle)
	}
}

// The original test drove on/off/on with five-second sleeps so a human could
// watch the plug; this asserts the command sequence instead.
func TestOnOffSequence(t *testing.T) {
	ep, b := newEzPlug(t)
	topic := ep.Topic(testEzPlugID)

	for _, step := range []struct {
		name string
		fn   func(string) error
	}{
		{"On", ep.On},
		{"Off", ep.Off},
		{"Toggle", ep.Toggle},
		{"On", ep.On},
	} {
		if err := step.fn(topic); err != nil {
			t.Fatalf("%s: %v", step.name, err)
		}
	}

	b.WaitForTopic(t, topic, 4, 5*time.Second)
	got := b.Payloads(topic)
	want := []string{On, Off, Toggle, On}
	if len(got) != len(want) {
		t.Fatalf("payloads = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("payloads = %v, want %v", got, want)
		}
	}
}

// Commands are published at QoS 0 and unretained; a retained power command
// would replay to any device that later subscribes.
func TestCommandsAreUnretainedQoS0(t *testing.T) {
	ep, b := newEzPlug(t)
	topic := ep.Topic(testEzPlugID)

	if err := ep.On(topic); err != nil {
		t.Fatalf("On: %v", err)
	}

	msgs := b.WaitForTopic(t, topic, 1, 5*time.Second)
	if msgs[0].QoS != 0 {
		t.Errorf("qos = %d, want 0", msgs[0].QoS)
	}
	if msgs[0].Retained {
		t.Error("retained = true, want false")
	}
}

func TestPublishFailsWhenDisconnected(t *testing.T) {
	// No broker: port 1 on loopback refuses immediately. Deliberately not using
	// an embedded broker here -- a client that never connects may retry in the
	// background, and an accept racing the broker's shutdown trips a data race
	// inside mochi-mqtt v2.7.9: attachClient runs Listeners.ClientsWg.Add(1)
	// from the connection goroutine while Server.Close -> Listeners.CloseAll
	// is already in ClientsWg.Wait(), which is the documented WaitGroup misuse
	// (upstream mochi-mqtt/server#424, fixed on release/2.8.0 by moving
	// registration into Listeners.Establish behind a shutdown flag).
	c := iot.New("tcp://127.0.0.1:1", testUsername, testPassword, testClientID, true)
	if _, err := c.Connect(); err == nil {
		t.Fatal("Connect to unreachable broker: err = nil, want error")
	}
	ep := New(c)

	if err := ep.On(ep.Topic(testEzPlugID)); err == nil {
		t.Fatal("On with no connection: err = nil, want error")
	}
}
