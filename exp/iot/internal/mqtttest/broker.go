// Package mqtttest provides an embedded MQTT broker for the exp/iot tests.
//
// The iot packages previously required a broker on the author's LAN: their
// tests hardcoded a broker address plus credentials, skipped themselves in CI,
// and locally burned a full TCP timeout before skipping. Running a real broker
// in-process instead means the paho client, the wire protocol and the published
// payloads are all genuinely exercised, with no network and no credentials.
package mqtttest

import (
	"context"
	"io"
	"log/slog"
	"net"
	"sync"
	"time"

	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/listeners"
	"github.com/mochi-mqtt/server/v2/packets"
)

// TB is the subset of *testing.T this package needs. Depending on an interface
// rather than *testing.T keeps the testing package out of a non-test import
// path, where it would register its flags into any binary that linked it.
type TB interface {
	Helper()
	Fatalf(format string, args ...any)
	Cleanup(func())
}

// Message is one publish observed by the broker.
type Message struct {
	Topic    string
	Payload  string
	ClientID string
	QoS      byte
	Retained bool
}

// Credentials is a username/password pair.
type Credentials struct {
	Username string
	Password string
}

// Broker is an embedded MQTT broker listening on loopback. It records every
// publish it receives and every credential pair offered to it, so tests can
// assert on what the client actually put on the wire.
type Broker struct {
	addr string
	rec  *recorder
}

type options struct {
	creds *Credentials
}

// Option configures a Broker.
type Option func(*options)

// WithCredentials makes the broker reject any CONNECT that does not present
// this username and password. Without it, the broker accepts any client.
func WithCredentials(username, password string) Option {
	return func(o *options) {
		o.creds = &Credentials{Username: username, Password: password}
	}
}

// New starts a broker on an ephemeral loopback port and stops it during test
// cleanup.
func New(t TB, opts ...Option) *Broker {
	t.Helper()

	var cfg options
	for _, o := range opts {
		o(&cfg)
	}

	// Bind here and hand the live listener to the broker via listeners.NewNet.
	// Reserving a port and closing it so the broker can rebind leaves a window
	// in which another process can take that port -- and under go test ./...
	// the sibling iot packages run in parallel, each starting brokers of its
	// own. Losing that race meant a client reaching a different package's
	// broker, which accepted it because every broker here uses the same test
	// credentials, and asserting against a broker that had recorded nothing.
	var lc net.ListenConfig
	l, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("mqtttest: listen: %v", err)
	}
	addr := l.Addr().String()

	// The default logger writes broker chatter to stdout at info level.
	srv := mqtt.New(&mqtt.Options{
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	rec := &recorder{allow: cfg.creds}
	if err := srv.AddHook(rec, nil); err != nil {
		_ = l.Close()
		t.Fatalf("mqtttest: add hook: %v", err)
	}
	// NewNet serves the listener it is given; Init does not bind anything, so
	// the socket above is the one clients reach.
	if err := srv.AddListener(listeners.NewNet("mqtttest", l)); err != nil {
		_ = l.Close()
		t.Fatalf("mqtttest: add listener: %v", err)
	}

	// Serve starts the accept loops and returns; it does not block. Calling it
	// synchronously means a startup failure fails the test instead of being
	// discarded in a goroutine.
	if err := srv.Serve(); err != nil {
		_ = l.Close()
		t.Fatalf("mqtttest: serve: %v", err)
	}
	t.Cleanup(func() {
		// Let the broker finish removing disconnected clients before closing.
		// Server.Close -> Clients.GetByListener takes a read lock and then
		// calls Clients.Len, which takes the same read lock; a Clients.Delete
		// still in flight for a just-disconnected client is the pending writer
		// that wedges the second acquisition permanently (upstream
		// mochi-mqtt/server#488, fixed on release/2.8.0). paho's Disconnect
		// returns before the broker has done that removal, so waiting on the
		// client side alone is not enough.
		deadline := time.Now().Add(5 * time.Second)
		for srv.Clients.Len() > 0 && time.Now().Before(deadline) {
			time.Sleep(2 * time.Millisecond)
		}
		_ = srv.Close()
	})

	// No readiness wait is needed: the socket is listening from the moment
	// net.Listen returns, so a connection arriving before the accept loop runs
	// simply waits in the backlog.
	return &Broker{addr: "tcp://" + addr, rec: rec}
}

// Addr is the broker URL, in the tcp://host:port form paho expects.
func (b *Broker) Addr() string { return b.addr }

// Messages returns every publish the broker has received, in order.
func (b *Broker) Messages() []Message { return b.rec.snapshot() }

// MessagesOn returns the publishes received on exactly this topic, in order.
func (b *Broker) MessagesOn(topic string) []Message {
	all := b.rec.snapshot()
	out := make([]Message, 0, len(all))
	for _, m := range all {
		if m.Topic == topic {
			out = append(out, m)
		}
	}
	return out
}

// Payloads returns just the payloads published on a topic, in order, which is
// the common shape for asserting a command sequence.
func (b *Broker) Payloads(topic string) []string {
	msgs := b.MessagesOn(topic)
	out := make([]string, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, m.Payload)
	}
	return out
}

// AuthAttempts returns every credential pair offered to the broker, in order.
func (b *Broker) AuthAttempts() []Credentials { return b.rec.authSnapshot() }

// WaitForMessages blocks until at least n publishes have been received, or
// fails the test on timeout. Publishing is asynchronous even at QoS 0, so
// assertions need this rather than a bare sleep.
func (b *Broker) WaitForMessages(t TB, n int, timeout time.Duration) []Message {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		msgs := b.rec.snapshot()
		if len(msgs) >= n {
			return msgs
		}
		if time.Now().After(deadline) {
			t.Fatalf("mqtttest: timed out after %s waiting for %d messages, saw %d: %+v",
				timeout, n, len(msgs), msgs)
			return msgs
		}
		time.Sleep(2 * time.Millisecond)
	}
}

// WaitForTopic blocks until at least n publishes have arrived on a topic.
func (b *Broker) WaitForTopic(t TB, topic string, n int, timeout time.Duration) []Message {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		msgs := b.MessagesOn(topic)
		if len(msgs) >= n {
			return msgs
		}
		if time.Now().After(deadline) {
			t.Fatalf("mqtttest: timed out after %s waiting for %d messages on %q, saw %d: %+v",
				timeout, n, topic, len(msgs), msgs)
			return msgs
		}
		time.Sleep(2 * time.Millisecond)
	}
}

// recorder is a mochi hook that authenticates clients and records traffic.
type recorder struct {
	mqtt.HookBase
	// allow is nil when the broker accepts any credentials.
	allow *Credentials

	mu       sync.Mutex
	messages []Message
	auths    []Credentials
}

func (r *recorder) ID() string { return "mqtttest-recorder" }

func (r *recorder) Provides(b byte) bool {
	switch b {
	case mqtt.OnConnectAuthenticate, mqtt.OnACLCheck, mqtt.OnPublish:
		return true
	default:
		return false
	}
}

func (r *recorder) OnConnectAuthenticate(_ *mqtt.Client, pk packets.Packet) bool {
	got := Credentials{
		Username: string(pk.Connect.Username),
		Password: string(pk.Connect.Password),
	}
	r.mu.Lock()
	r.auths = append(r.auths, got)
	r.mu.Unlock()

	if r.allow == nil {
		return true
	}
	return got == *r.allow
}

// OnACLCheck permits every topic; these tests are about the client, not
// authorization.
func (r *recorder) OnACLCheck(_ *mqtt.Client, _ string, _ bool) bool { return true }

// OnPublish records on receipt rather than on delivery, so a publish with no
// subscribers is still observed.
func (r *recorder) OnPublish(cl *mqtt.Client, pk packets.Packet) (packets.Packet, error) {
	r.mu.Lock()
	r.messages = append(r.messages, Message{
		Topic:    pk.TopicName,
		Payload:  string(pk.Payload),
		ClientID: cl.ID,
		QoS:      pk.FixedHeader.Qos,
		Retained: pk.FixedHeader.Retain,
	})
	r.mu.Unlock()
	return pk, nil
}

func (r *recorder) snapshot() []Message {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]Message(nil), r.messages...)
}

func (r *recorder) authSnapshot() []Credentials {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]Credentials(nil), r.auths...)
}

// assert at compile time that recorder satisfies the hook contract.
var _ mqtt.Hook = (*recorder)(nil)
