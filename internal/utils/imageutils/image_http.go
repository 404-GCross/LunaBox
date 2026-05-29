package imageutils

import (
	"net/http"
	"time"

	"lunabox/internal/utils/proxyutils"
)

func newImageHTTPClient(timeout time.Duration, mode string, manualURL string) (*http.Client, error) {
	client, _, err := proxyutils.NewHTTPClient(timeout, mode, manualURL)
	return client, err
}

func newImageHTTPClientFromConfig(timeout time.Duration, config proxyutils.ProxyConfigProvider) (*http.Client, error) {
	client, _, err := proxyutils.NewHTTPClientFromConfig(timeout, config)
	return client, err
}

func newSystemImageHTTPClient(timeout time.Duration) (*http.Client, error) {
	client, _, err := proxyutils.NewSystemHTTPClient(timeout)
	return client, err
}
