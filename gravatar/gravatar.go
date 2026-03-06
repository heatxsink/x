package gravatar

import (
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"strconv"
	"strings"
)

const (
	baseHost   = "gravatar.com"
	apiHost    = "api.gravatar.com"
	avatarPath = "/avatar/"
	qrCodePath = "/v3/qr-code/"
)

// DefaultImage specifies the fallback image when no gravatar exists.
type DefaultImage string

const (
	Default404       DefaultImage = "404"
	DefaultMP        DefaultImage = "mp"
	DefaultIdenticon DefaultImage = "identicon"
	DefaultMonster   DefaultImage = "monsterid"
	DefaultWavatar   DefaultImage = "wavatar"
	DefaultRetro     DefaultImage = "retro"
	DefaultRobot     DefaultImage = "robohash"
	DefaultBlank     DefaultImage = "blank"
)

// Rating specifies the maximum content rating for avatar images.
type Rating string

const (
	RatingG  Rating = "g"
	RatingPG Rating = "pg"
	RatingR  Rating = "r"
)

// ProfileFormat specifies the response format for profile requests.
type ProfileFormat string

const (
	FormatJSON ProfileFormat = "json"
	FormatXML  ProfileFormat = "xml"
	FormatVCF  ProfileFormat = "vcf"
)

type avatarConfig struct {
	size         int
	defaultImage DefaultImage
	rating       Rating
	forceDefault bool
}

// AvatarOption configures avatar URL generation.
type AvatarOption func(*avatarConfig)

// WithSize sets the avatar image size in pixels (1-2048).
func WithSize(px int) AvatarOption {
	return func(c *avatarConfig) {
		if px >= 1 && px <= 2048 {
			c.size = px
		}
	}
}

// WithDefault sets the fallback image type.
func WithDefault(d DefaultImage) AvatarOption {
	return func(c *avatarConfig) {
		c.defaultImage = d
	}
}

// WithDefaultURL sets a custom fallback image URL.
func WithDefaultURL(u string) AvatarOption {
	return func(c *avatarConfig) {
		c.defaultImage = DefaultImage(u)
	}
}

// WithRating sets the maximum content rating.
func WithRating(r Rating) AvatarOption {
	return func(c *avatarConfig) {
		c.rating = r
	}
}

// WithForceDefault forces the default image regardless of whether a gravatar exists.
func WithForceDefault() AvatarOption {
	return func(c *avatarConfig) {
		c.forceDefault = true
	}
}

// Hash returns the SHA256 hash of a trimmed, lowercased email address.
func Hash(email string) string {
	e := strings.ToLower(strings.TrimSpace(email))
	h := sha256.Sum256([]byte(e))
	return hex.EncodeToString(h[:])
}

// AvatarURL builds a Gravatar avatar image URL with the given options.
func AvatarURL(email string, opts ...AvatarOption) string {
	cfg := avatarConfig{
		size:         80,
		defaultImage: DefaultMP,
		rating:       RatingG,
	}
	for _, o := range opts {
		o(&cfg)
	}
	v := url.Values{}
	v.Set("s", strconv.Itoa(cfg.size))
	v.Set("d", string(cfg.defaultImage))
	v.Set("r", string(cfg.rating))
	if cfg.forceDefault {
		v.Set("f", "y")
	}
	u := url.URL{
		Scheme:   "https",
		Host:     baseHost,
		Path:     avatarPath + Hash(email),
		RawQuery: v.Encode(),
	}
	return u.String()
}

// ProfileURL builds a Gravatar profile URL in the specified format.
func ProfileURL(email string, format ProfileFormat) string {
	u := url.URL{
		Scheme: "https",
		Host:   baseHost,
		Path:   "/" + Hash(email) + "." + string(format),
	}
	return u.String()
}

// QRCodeURL builds a Gravatar QR code URL via the v3 API.
func QRCodeURL(email string) string {
	u := url.URL{
		Scheme: "https",
		Host:   apiHost,
		Path:   qrCodePath + Hash(email),
	}
	return u.String()
}

// GetURL is the legacy API. It returns an avatar URL with the given size,
// rating=g, and default=retro.
func GetURL(email string, size int) string {
	return AvatarURL(email, WithSize(size), WithDefault(DefaultRetro), WithRating(RatingG))
}
