package appconf

import (
	"reflect"
	"testing"
)

func TestNormalizeMetadataSourcesAcceptsOptInSources(t *testing.T) {
	got := normalizeMetadataSources([]string{"bangumi", "dlsite", "erogamescape", "DLSITE", "unknown"})
	want := []string{"bangumi", "dlsite", "erogamescape"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %#v, got %#v", want, got)
	}
}

func TestNormalizeMetadataSourcesDefaultsDoNotIncludeOptInSources(t *testing.T) {
	got := normalizeMetadataSources(nil)

	for _, source := range got {
		if source == "dlsite" || source == "erogamescape" {
			t.Fatalf("opt-in source %q should not be enabled by default: %#v", source, got)
		}
	}
}

func TestNormalizeProxySettingsKeepsNetworkProxyURLAsGlobalURL(t *testing.T) {
	config := &AppConfig{
		NetworkProxyMode: "manual",
		NetworkProxyURL:  " 127.0.0.1:7890 ",
	}

	if !NormalizeProxySettings(config) {
		t.Fatal("expected proxy normalization to report changes")
	}
	if config.NetworkProxyURL != "127.0.0.1:7890" {
		t.Fatalf("expected global proxy URL to be trimmed, got %q", config.NetworkProxyURL)
	}
	if config.NetworkProxyMode != "manual" {
		t.Fatalf("unexpected proxy mode: %q", config.NetworkProxyMode)
	}
}

func TestNetworkProxyConfigReturnsGlobalProxy(t *testing.T) {
	config := &AppConfig{
		NetworkProxyMode: "manual",
		NetworkProxyURL:  "http://127.0.0.1:7890",
	}

	mode, proxyURL := config.NetworkProxyConfig()
	if mode != "manual" || proxyURL != config.NetworkProxyURL {
		t.Fatalf("unexpected network proxy config: mode=%q url=%q", mode, proxyURL)
	}
}
