package httputils

import (
	"net/http"
	"strings"
	"time"

	"lunabox/internal/utils/proxyutils"
	"lunabox/internal/version"
)

// ClientOptions configures a standard application HTTP client.
type ClientOptions struct {
	Timeout     time.Duration
	ProxyConfig proxyutils.ProxyConfigProvider
	ProxyMode   string
	ProxyURL    string
	UserAgent   string
}

// NewClient creates an HTTP client with application proxy settings and a
// default User-Agent. ProxyConfig takes precedence over ProxyMode and ProxyURL.
func NewClient(options ClientOptions) (*http.Client, string, error) {
	var (
		client           *http.Client
		proxyDescription string
		err              error
	)
	if options.ProxyConfig != nil {
		client, proxyDescription, err = proxyutils.NewHTTPClientFromConfig(options.Timeout, options.ProxyConfig)
	} else {
		client, proxyDescription, err = proxyutils.NewHTTPClient(options.Timeout, options.ProxyMode, options.ProxyURL)
	}
	if err != nil {
		return nil, "", err
	}

	userAgent := strings.TrimSpace(options.UserAgent)
	if userAgent == "" {
		userAgent = version.UserAgent()
	}
	client.Transport = &defaultUserAgentTransport{
		base:      client.Transport,
		userAgent: userAgent,
	}
	return client, proxyDescription, nil
}

type defaultUserAgentTransport struct {
	base      http.RoundTripper
	userAgent string
}

func (t *defaultUserAgentTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	cloned := req.Clone(req.Context())
	cloned.Header = req.Header.Clone()
	if cloned.Header == nil {
		cloned.Header = make(http.Header)
	}
	if cloned.Header.Get("User-Agent") == "" {
		cloned.Header.Set("User-Agent", t.userAgent)
	}

	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	return base.RoundTrip(cloned)
}

func (t *defaultUserAgentTransport) CloseIdleConnections() {
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	if closer, ok := base.(interface{ CloseIdleConnections() }); ok {
		closer.CloseIdleConnections()
	}
}
