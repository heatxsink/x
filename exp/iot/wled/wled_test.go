package wled

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/heatxsink/x/exp/iot"
	"github.com/heatxsink/x/exp/iot/internal/mqtttest"
)

const (
	testUsername = "test-user"
	testPassword = "test-password"
	testClientID = "wled-tests"
)

// newWLed returns a WLed wired to an embedded broker, plus that broker so the
// caller can assert on what was published.
func newWLed(t *testing.T) (*WLed, *mqtttest.Broker) {
	t.Helper()
	b := mqtttest.New(t, mqtttest.WithCredentials(testUsername, testPassword))
	c := iot.New(b.Addr(), testUsername, testPassword, testClientID, true)
	if _, err := c.Connect(); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	return New(c), b
}

func assertPayloads(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("payloads = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("payloads = %v, want %v", got, want)
		}
	}
}

// --- state commands ----------------------------------------------------------

func TestOnOffToggle(t *testing.T) {
	w, b := newWLed(t)

	for _, step := range []struct {
		name string
		fn   func(string) error
	}{
		{"On", w.On},
		{"Off", w.Off},
		{"Toggle", w.Toggle},
	} {
		if err := step.fn(TopicAll); err != nil {
			t.Fatalf("%s: %v", step.name, err)
		}
	}

	b.WaitForTopic(t, TopicAll, 3, 5*time.Second)
	assertPayloads(t, b.Payloads(TopicAll), []string{On, Off, Toggle})
}

func TestBrightness(t *testing.T) {
	w, b := newWLed(t)

	if err := w.Brightness(TopicAll, 128); err != nil {
		t.Fatalf("Brightness: %v", err)
	}

	msgs := b.WaitForTopic(t, TopicAll, 1, 5*time.Second)
	if msgs[0].Payload != "128" {
		t.Errorf("payload = %q, want %q", msgs[0].Payload, "128")
	}
}

// Brightness formats its value as a base-10 string, including the edges.
func TestBrightnessRange(t *testing.T) {
	w, b := newWLed(t)

	want := make([]string, 0, 26)
	for i := 10; i <= 255; i += 10 {
		if err := w.Brightness(TopicAll, int64(i)); err != nil {
			t.Fatalf("Brightness(%d): %v", i, err)
		}
		want = append(want, strconv.Itoa(i))
	}

	b.WaitForTopic(t, TopicAll, len(want), 10*time.Second)
	assertPayloads(t, b.Payloads(TopicAll), want)
}

func TestBrightnessNegative(t *testing.T) {
	w, b := newWLed(t)

	if err := w.Brightness(TopicAll, -1); err != nil {
		t.Fatalf("Brightness: %v", err)
	}

	msgs := b.WaitForTopic(t, TopicAll, 1, 5*time.Second)
	if msgs[0].Payload != "-1" {
		t.Errorf("payload = %q, want %q", msgs[0].Payload, "-1")
	}
}

// Color publishes to a /col subtopic, not the base topic.
func TestColorUsesColSubtopic(t *testing.T) {
	w, b := newWLed(t)

	if err := w.Color(TopicAll, "#FC419A"); err != nil {
		t.Fatalf("Color: %v", err)
	}

	msgs := b.WaitForTopic(t, TopicAll+"/col", 1, 5*time.Second)
	if msgs[0].Payload != "#FC419A" {
		t.Errorf("payload = %q, want %q", msgs[0].Payload, "#FC419A")
	}
	if base := b.MessagesOn(TopicAll); len(base) != 0 {
		t.Errorf("Color published to the base topic as well: %+v", base)
	}
}

func TestColorPresets(t *testing.T) {
	w, b := newWLed(t)
	presets := []string{"#FC419A", "#FF0000", "#00FF00", "#0000FF", "#FBFBF8", "#1E90FF", "#FE5A1D", "#ED008C"}

	for _, c := range presets {
		if err := w.Color(TopicAll, c); err != nil {
			t.Fatalf("Color(%q): %v", c, err)
		}
	}

	b.WaitForTopic(t, TopicAll+"/col", len(presets), 10*time.Second)
	assertPayloads(t, b.Payloads(TopicAll+"/col"), presets)
}

// API publishes to a /api subtopic, not the base topic.
func TestAPIUsesAPISubtopic(t *testing.T) {
	w, b := newWLed(t)

	for _, cmd := range []string{"FX=0", "FX=91&SX=210&IX=128"} {
		if err := w.API(TopicAll, cmd); err != nil {
			t.Fatalf("API(%q): %v", cmd, err)
		}
	}

	b.WaitForTopic(t, TopicAll+"/api", 2, 5*time.Second)
	assertPayloads(t, b.Payloads(TopicAll+"/api"), []string{"FX=0", "FX=91&SX=210&IX=128"})
}

// PulseN(topic, 1) publishes On, then alternates Off and On for two iterations.
func TestPulseN(t *testing.T) {
	w, b := newWLed(t)

	if err := w.PulseN(TopicAll, 1); err != nil {
		t.Fatalf("PulseN: %v", err)
	}

	b.WaitForTopic(t, TopicAll, 3, 5*time.Second)
	assertPayloads(t, b.Payloads(TopicAll), []string{On, Off, On})
}

func TestPulseNZeroOnlyTurnsOn(t *testing.T) {
	w, b := newWLed(t)

	if err := w.PulseN(TopicAll, 0); err != nil {
		t.Fatalf("PulseN: %v", err)
	}

	b.WaitForTopic(t, TopicAll, 1, 5*time.Second)
	assertPayloads(t, b.Payloads(TopicAll), []string{On})
}

func TestCommandsFailWhenDisconnected(t *testing.T) {
	b := mqtttest.New(t, mqtttest.WithCredentials(testUsername, testPassword))
	c := iot.New(b.Addr(), testUsername, "wrong-password", testClientID, true)
	if _, err := c.Connect(); err == nil {
		t.Fatal("Connect with wrong password: err = nil, want error")
	}
	w := New(c)

	if err := w.On(TopicAll); err == nil {
		t.Fatal("On with no connection: err = nil, want error")
	}
	if err := w.Color(TopicAll, "#FF0000"); err == nil {
		t.Fatal("Color with no connection: err = nil, want error")
	}
}

// --- Info (HTTP) -------------------------------------------------------------

const infoJSON = `{
  "state": {"on": true, "bri": 128, "seg": [{"id": 0, "bri": 200, "col": [[255, 0, 0]], "fx": 91}]},
  "info": {"ver": "0.14.0", "name": "desk", "leds": {"count": 60, "fps": 42}, "ip": "10.0.0.9"},
  "effects": ["Solid", "Blink"],
  "palettes": ["Default", "Party"]
}`

// hostOf strips the scheme, since Info builds its own http:// URL.
func hostOf(t *testing.T, srv *httptest.Server) string {
	t.Helper()
	return strings.TrimPrefix(srv.URL, "http://")
}

func TestInfo(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write([]byte(infoJSON)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer srv.Close()

	li, err := Info(context.Background(), hostOf(t, srv))
	if err != nil {
		t.Fatalf("Info: %v", err)
	}

	if gotPath != "/json/si" {
		t.Errorf("requested path = %q, want /json/si", gotPath)
	}
	if !li.State.On {
		t.Error("State.On = false, want true")
	}
	if li.State.Bri != 128 {
		t.Errorf("State.Bri = %d, want 128", li.State.Bri)
	}
	if li.Info.Name != "desk" {
		t.Errorf("Info.Name = %q, want desk", li.Info.Name)
	}
	if li.Info.Leds.Count != 60 {
		t.Errorf("Info.Leds.Count = %d, want 60", li.Info.Leds.Count)
	}
	if len(li.State.Seg) != 1 || li.State.Seg[0].Fx != 91 {
		t.Errorf("State.Seg = %+v, want one segment with Fx 91", li.State.Seg)
	}
	if len(li.Effects) != 2 || li.Effects[0] != "Solid" {
		t.Errorf("Effects = %v, want [Solid Blink]", li.Effects)
	}
}

func TestInfoNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	_, err := Info(context.Background(), hostOf(t, srv))
	if err == nil {
		t.Fatal("Info on 500: err = nil, want error")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error = %v, want it to name the status code", err)
	}
}

func TestInfoMalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if _, err := w.Write([]byte("{not json")); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer srv.Close()

	if _, err := Info(context.Background(), hostOf(t, srv)); err == nil {
		t.Fatal("Info on malformed JSON: err = nil, want error")
	}
}

func TestInfoCanceledContext(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if _, err := w.Write([]byte(infoJSON)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer srv.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := Info(ctx, hostOf(t, srv)); err == nil {
		t.Fatal("Info with canceled context: err = nil, want error")
	}
}

func TestInfoUnreachableHost(t *testing.T) {
	// Port 1 on loopback refuses immediately.
	if _, err := Info(context.Background(), "127.0.0.1:1"); err == nil {
		t.Fatal("Info against unreachable host: err = nil, want error")
	}
}
