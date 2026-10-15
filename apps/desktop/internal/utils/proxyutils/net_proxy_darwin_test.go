//go:build darwin

package proxyutils

import (
	"net/http"
	"testing"
)

func TestParseDarwinScutilProxyOutputStaticProxy(t *testing.T) {
	raw := `<dictionary> {
  ExceptionsList : <array> {
    0 : *.local
    1 : 169.254/16
  }
  HTTPEnable : 1
  HTTPPort : 7890
  HTTPProxy : 127.0.0.1
  HTTPSEnable : 1
  HTTPSPort : 7891
  HTTPSProxy : proxy.example.test
  SOCKSEnable : 1
  SOCKSPort : 1080
  SOCKSProxy : socks.example.test
}`

	selection, note, err := parseDarwinScutilProxyOutput(raw)
	if err != nil {
		t.Fatalf("parseDarwinScutilProxyOutput() error = %v", err)
	}
	if note != "" {
		t.Fatalf("note = %q, want empty", note)
	}
	if selection == nil || !selection.HasProxy() {
		t.Fatal("selection has no proxy")
	}
	if got := selection.HTTPProxy.String(); got != "http://127.0.0.1:7890" {
		t.Fatalf("HTTPProxy = %q", got)
	}
	if got := selection.HTTPSProxy.String(); got != "http://proxy.example.test:7891" {
		t.Fatalf("HTTPSProxy = %q", got)
	}
	if got := selection.AllProxy.String(); got != "socks5://socks.example.test:1080" {
		t.Fatalf("AllProxy = %q", got)
	}

	req, _ := http.NewRequest(http.MethodGet, "https://demo.local/path", nil)
	proxyURL, err := selection.Proxy(req)
	if err != nil {
		t.Fatalf("Proxy() error = %v", err)
	}
	if proxyURL != nil {
		t.Fatalf("Proxy() for bypassed host = %v, want nil", proxyURL)
	}
}

func TestParseDarwinScutilProxyOutputPACOnly(t *testing.T) {
	raw := `<dictionary> {
  ProxyAutoConfigEnable : 1
  ProxyAutoConfigURLString : http://proxy.example.test/proxy.pac
}`

	selection, note, err := parseDarwinScutilProxyOutput(raw)
	if err != nil {
		t.Fatalf("parseDarwinScutilProxyOutput() error = %v", err)
	}
	if selection != nil {
		t.Fatalf("selection = %#v, want nil", selection)
	}
	if note != "PAC detected" {
		t.Fatalf("note = %q, want PAC detected", note)
	}
}
