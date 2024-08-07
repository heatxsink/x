package throttled

import (
	"net/http"
	"time"

	"golang.org/x/time/rate"
)

type ThrottledTransport struct {
	roundTripperWrap http.RoundTripper
	rateLimiter      *rate.Limiter
}

func (c *ThrottledTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	err := c.rateLimiter.Wait(r.Context()) // This is a blocking call. Honors the rate limit
	if err != nil {
		return nil, err
	}
	return c.roundTripperWrap.RoundTrip(r)
}

// Example usage:
// client := http.DefaultClient
// client.Transport = NewThrottledTransport(10*time.Seconds, 60, http.DefaultTransport) allows 60 requests every 10 seconds
func NewThrottledTransport(limitPeriod time.Duration, requestCount int, transportWrap http.RoundTripper) http.RoundTripper {
	return &ThrottledTransport{
		roundTripperWrap: transportWrap,
		rateLimiter:      rate.NewLimiter(rate.Every(limitPeriod), requestCount),
	}
}
