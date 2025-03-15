package gravatar

import (
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"strconv"
	"strings"
)

func GetURL(email string, size int) string {
	hasher := sha256.Sum256([]byte(strings.TrimSpace(email)))
	hash := hex.EncodeToString(hasher[:])
	path := "/avatar/" + hash
	v := url.Values{}
	v.Add("s", strconv.Itoa(size))
	v.Add("r", "g")
	v.Add("d", "retro")
	url := url.URL{
		Scheme:   "https",
		Host:     "www.gravatar.com",
		Path:     path,
		RawQuery: v.Encode(),
	}
	return url.String()
}
