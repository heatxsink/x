package gravatar

import (
	"strings"
	"testing"
)

const testEmail = "ngranado@gmail.com"

func TestHash(t *testing.T) {
	h := Hash(testEmail)
	if len(h) != 64 {
		t.Fatalf("expected 64 char hex string, got %d", len(h))
	}
	// Same email with whitespace and caps should produce the same hash.
	if Hash("  NGRANADO@gmail.com  ") != h {
		t.Fatal("hash should normalize whitespace and case")
	}
}

func TestAvatarURLDefaults(t *testing.T) {
	u := AvatarURL(testEmail)
	if !strings.HasPrefix(u, "https://gravatar.com/avatar/") {
		t.Fatalf("unexpected prefix: %s", u)
	}
	if !strings.Contains(u, "s=80") {
		t.Errorf("expected default size 80: %s", u)
	}
	if !strings.Contains(u, "d=mp") {
		t.Errorf("expected default mp: %s", u)
	}
	if !strings.Contains(u, "r=g") {
		t.Errorf("expected rating g: %s", u)
	}
	if strings.Contains(u, "f=y") {
		t.Errorf("force default should not be set by default: %s", u)
	}
}

func TestAvatarURLWithOptions(t *testing.T) {
	u := AvatarURL(testEmail,
		WithSize(200),
		WithDefault(DefaultIdenticon),
		WithRating(RatingPG),
		WithForceDefault(),
	)
	if !strings.Contains(u, "s=200") {
		t.Errorf("expected size 200: %s", u)
	}
	if !strings.Contains(u, "d=identicon") {
		t.Errorf("expected default identicon: %s", u)
	}
	if !strings.Contains(u, "r=pg") {
		t.Errorf("expected rating pg: %s", u)
	}
	if !strings.Contains(u, "f=y") {
		t.Errorf("expected force default: %s", u)
	}
}

func TestAvatarURLWithDefaultURL(t *testing.T) {
	u := AvatarURL(testEmail, WithDefaultURL("https://example.com/avatar.png"))
	if !strings.Contains(u, "d=https") {
		t.Errorf("expected custom default URL: %s", u)
	}
}

func TestAvatarURLSizeClamping(t *testing.T) {
	u := AvatarURL(testEmail, WithSize(0))
	if !strings.Contains(u, "s=80") {
		t.Errorf("size 0 should keep default 80: %s", u)
	}
	u = AvatarURL(testEmail, WithSize(9999))
	if !strings.Contains(u, "s=80") {
		t.Errorf("size 9999 should keep default 80: %s", u)
	}
	u = AvatarURL(testEmail, WithSize(2048))
	if !strings.Contains(u, "s=2048") {
		t.Errorf("size 2048 should be accepted: %s", u)
	}
}

func TestProfileURL(t *testing.T) {
	tests := []struct {
		format ProfileFormat
		ext    string
	}{
		{FormatJSON, ".json"},
		{FormatXML, ".xml"},
		{FormatVCF, ".vcf"},
	}
	for _, tt := range tests {
		u := ProfileURL(testEmail, tt.format)
		if !strings.HasPrefix(u, "https://gravatar.com/") {
			t.Errorf("unexpected prefix for %s: %s", tt.format, u)
		}
		if !strings.HasSuffix(u, tt.ext) {
			t.Errorf("expected suffix %s: %s", tt.ext, u)
		}
	}
}

func TestQRCodeURL(t *testing.T) {
	u := QRCodeURL(testEmail)
	if !strings.HasPrefix(u, "https://api.gravatar.com/v3/qr-code/") {
		t.Fatalf("unexpected prefix: %s", u)
	}
	if !strings.Contains(u, Hash(testEmail)) {
		t.Fatalf("expected hash in URL: %s", u)
	}
}

func TestGetURLBackwardCompatibility(t *testing.T) {
	u := GetURL(testEmail, 100)
	if !strings.Contains(u, "s=100") {
		t.Errorf("expected size 100: %s", u)
	}
	if !strings.Contains(u, "d=retro") {
		t.Errorf("expected default retro: %s", u)
	}
	if !strings.Contains(u, "r=g") {
		t.Errorf("expected rating g: %s", u)
	}
}

func TestAllDefaultImages(t *testing.T) {
	defaults := []DefaultImage{
		Default404, DefaultMP, DefaultIdenticon, DefaultMonster,
		DefaultWavatar, DefaultRetro, DefaultRobot, DefaultBlank,
	}
	for _, d := range defaults {
		u := AvatarURL(testEmail, WithDefault(d))
		if !strings.Contains(u, "d="+string(d)) {
			t.Errorf("expected d=%s in URL: %s", d, u)
		}
	}
}

func TestAllRatings(t *testing.T) {
	ratings := []Rating{RatingG, RatingPG, RatingR}
	for _, r := range ratings {
		u := AvatarURL(testEmail, WithRating(r))
		if !strings.Contains(u, "r="+string(r)) {
			t.Errorf("expected r=%s in URL: %s", r, u)
		}
	}
}
