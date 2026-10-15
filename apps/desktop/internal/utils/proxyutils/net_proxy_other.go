//go:build !windows && !darwin

package proxyutils

func loadSystemProxySelection() (*ProxySelection, string, error) {
	return nil, "", nil
}
