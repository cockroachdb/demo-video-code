package http

import (
	"fmt"
	"net/http"
	"net/url"
)

// NewClient returns a pointer to a new instance of http.Client with
// an underlying prefix transporter, allowing for a base URL to be
// provided, with all sub paths being relative to that.
func NewClient(baseURL string) *http.Client {
	transport := &urlPrefixTransport{
		BaseURL: baseURL,
	}

	client := &http.Client{
		Transport: transport,
	}

	return client
}

type urlPrefixTransport struct {
	BaseURL string
}

func (t *urlPrefixTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	base, err := url.Parse(t.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("parsing base url: %w", err)
	}

	u, err := url.Parse(req.URL.String())
	if err != nil {
		return nil, fmt.Errorf("parsing request url: %w", err)
	}

	u = base.ResolveReference(u)
	req.URL = u
	req.Header.Add("Content-Type", "application/json")

	return http.DefaultTransport.RoundTrip(req)
}
