package iot

import (
	"sync"
	"testing"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/heatxsink/x/exp/iot/internal/mqtttest"
)

const (
	testUsername = "test-user"
	testPassword = "test-password"
	testClientID = "iot-tests"
)

// connected returns a client already connected to the broker.
func connected(t *testing.T, b *mqtttest.Broker, clientID string) *IoT {
	t.Helper()
	c := New(b.Addr(), testUsername, testPassword, clientID, true)
	if _, err := c.Connect(); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	return c
}

func TestNewSetsClientOptions(t *testing.T) {
	c := New("tcp://broker.example:1883", testUsername, testPassword, testClientID, true)

	if c.client == nil {
		t.Fatal("client is nil")
	}
	opts := c.opts
	if got := opts.Servers; len(got) != 1 || got[0].String() != "tcp://broker.example:1883" {
		t.Errorf("Servers = %v, want [tcp://broker.example:1883]", got)
	}
	if opts.Username != testUsername {
		t.Errorf("Username = %q, want %q", opts.Username, testUsername)
	}
	if opts.Password != testPassword {
		t.Errorf("Password = %q, want %q", opts.Password, testPassword)
	}
	if opts.ClientID != testClientID {
		t.Errorf("ClientID = %q, want %q", opts.ClientID, testClientID)
	}
	if !opts.CleanSession {
		t.Error("CleanSession = false, want true")
	}
}

func TestConnect(t *testing.T) {
	b := mqtttest.New(t, mqtttest.WithCredentials(testUsername, testPassword))
	c := connected(t, b, testClientID)
	defer c.client.Disconnect(250)

	if !c.client.IsConnected() {
		t.Error("IsConnected = false after Connect")
	}
	attempts := b.AuthAttempts()
	if len(attempts) != 1 {
		t.Fatalf("auth attempts = %d, want 1: %+v", len(attempts), attempts)
	}
	if attempts[0].Username != testUsername || attempts[0].Password != testPassword {
		t.Errorf("broker saw %+v, want username %q password %q",
			attempts[0], testUsername, testPassword)
	}
}

// The broker rejects the CONNECT, and that rejection must reach the caller
// rather than being swallowed into a nil error.
func TestConnectWrongCredentials(t *testing.T) {
	b := mqtttest.New(t, mqtttest.WithCredentials(testUsername, testPassword))
	c := New(b.Addr(), testUsername, "wrong-password", testClientID, true)

	if _, err := c.Connect(); err == nil {
		t.Fatal("Connect with wrong password: err = nil, want error")
	}
	// Stop the client before the broker is torn down: a lingering reconnect
	// attempt accepted during shutdown trips a data race inside mochi-mqtt
	// v2.7.9 (Server.Close closes s.done while Server.NewClient reads it).
	c.client.Disconnect(0)

	if c.client.IsConnected() {
		t.Error("IsConnected = true after a rejected Connect")
	}
	// The broker saw and rejected the attempt. paho retries a rejected CONNECT,
	// so the count is not fixed -- assert only that every attempt it made
	// carried the credentials under test.
	attempts := b.AuthAttempts()
	if len(attempts) == 0 {
		t.Fatal("broker recorded no authentication attempts")
	}
	for i, a := range attempts {
		if a.Username != testUsername || a.Password != "wrong-password" {
			t.Errorf("attempt %d = %+v, want username %q with the wrong password",
				i, a, testUsername)
		}
	}
}

func TestConnectUnreachableBroker(t *testing.T) {
	// Port 1 on loopback refuses immediately.
	c := New("tcp://127.0.0.1:1", testUsername, testPassword, testClientID, true)
	c.opts.SetConnectTimeout(2 * time.Second)

	if _, err := c.Connect(); err == nil {
		t.Fatal("Connect to unreachable broker: err = nil, want error")
	}
}

func TestPublish(t *testing.T) {
	b := mqtttest.New(t)
	c := connected(t, b, testClientID)
	defer c.client.Disconnect(250)

	if err := c.Publish("test/topic", 0, false, "the payload"); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	msgs := b.WaitForTopic(t, "test/topic", 1, 5*time.Second)
	if msgs[0].Payload != "the payload" {
		t.Errorf("payload = %q, want %q", msgs[0].Payload, "the payload")
	}
	if msgs[0].Retained {
		t.Error("retained = true, want false")
	}
	if msgs[0].ClientID != testClientID {
		t.Errorf("client id = %q, want %q", msgs[0].ClientID, testClientID)
	}
}

func TestPublishRetained(t *testing.T) {
	b := mqtttest.New(t)
	c := connected(t, b, testClientID)
	defer c.client.Disconnect(250)

	if err := c.Publish("test/retained", 0, true, "sticky"); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	msgs := b.WaitForTopic(t, "test/retained", 1, 5*time.Second)
	if !msgs[0].Retained {
		t.Error("retained = false, want true")
	}
}

func TestPublishQoS1(t *testing.T) {
	b := mqtttest.New(t)
	c := connected(t, b, testClientID)
	defer c.client.Disconnect(250)

	if err := c.Publish("test/qos", 1, false, "at least once"); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	msgs := b.WaitForTopic(t, "test/qos", 1, 5*time.Second)
	if msgs[0].QoS != 1 {
		t.Errorf("qos = %d, want 1", msgs[0].QoS)
	}
}

// Publish returns the token error rather than nil once the client is gone.
func TestPublishAfterDisconnect(t *testing.T) {
	b := mqtttest.New(t)
	c := connected(t, b, testClientID)
	c.client.Disconnect(250)

	if err := c.Publish("test/topic", 1, false, "into the void"); err == nil {
		t.Fatal("Publish after Disconnect: err = nil, want error")
	}
}

// Subscribe must deliver a matching publish to the handler.
func TestSubscribeReceivesPublish(t *testing.T) {
	b := mqtttest.New(t)
	sub := connected(t, b, "iot-tests-sub")
	defer sub.client.Disconnect(250)
	pub := connected(t, b, "iot-tests-pub")
	defer pub.client.Disconnect(250)

	var (
		mu       sync.Mutex
		received []string
	)
	done := make(chan struct{})
	err := sub.Subscribe("test/sub", 0, func(_ mqtt.Client, m mqtt.Message) {
		mu.Lock()
		received = append(received, string(m.Payload()))
		mu.Unlock()
		close(done)
	})
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	if err := pub.Publish("test/sub", 0, false, "delivered"); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("handler never received the message")
	}

	mu.Lock()
	defer mu.Unlock()
	if len(received) != 1 || received[0] != "delivered" {
		t.Errorf("received = %v, want [delivered]", received)
	}
}

// A wildcard subscription must match the topics beneath it.
func TestSubscribeWildcard(t *testing.T) {
	b := mqtttest.New(t)
	sub := connected(t, b, "iot-tests-sub")
	defer sub.client.Disconnect(250)
	pub := connected(t, b, "iot-tests-pub")
	defer pub.client.Disconnect(250)

	topics := make(chan string, 2)
	err := sub.Subscribe("devices/#", 0, func(_ mqtt.Client, m mqtt.Message) {
		topics <- m.Topic()
	})
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	for _, topic := range []string{"devices/a/power", "devices/b/power"} {
		if err := pub.Publish(topic, 0, false, "on"); err != nil {
			t.Fatalf("Publish %q: %v", topic, err)
		}
	}

	got := map[string]bool{}
	for range 2 {
		select {
		case topic := <-topics:
			got[topic] = true
		case <-time.After(5 * time.Second):
			t.Fatalf("timed out; received %v", got)
		}
	}
	for _, want := range []string{"devices/a/power", "devices/b/power"} {
		if !got[want] {
			t.Errorf("missing topic %q; received %v", want, got)
		}
	}
}

func TestSubscribeAfterDisconnect(t *testing.T) {
	b := mqtttest.New(t)
	c := connected(t, b, testClientID)
	c.client.Disconnect(250)

	err := c.Subscribe("test/topic", 0, func(mqtt.Client, mqtt.Message) {})
	if err == nil {
		t.Fatal("Subscribe after Disconnect: err = nil, want error")
	}
}
