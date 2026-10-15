//go:build darwin

package proxyutils

import (
	"fmt"
	"net"
	"net/url"
	"os/exec"
	"strconv"
	"strings"
)

const scutilProxyCommand = "scutil"

func loadSystemProxySelection() (*ProxySelection, string, error) {
	out, err := exec.Command(scutilProxyCommand, "--proxy").Output()
	if err != nil {
		return nil, "", fmt.Errorf("read macOS system proxy: %w", err)
	}
	return parseDarwinScutilProxyOutput(string(out))
}

func parseDarwinScutilProxyOutput(raw string) (*ProxySelection, string, error) {
	values, arrays := parseDarwinProxyDictionary(raw)
	selection := &ProxySelection{
		Source: "system",
		Bypass: arrays["ExceptionsList"],
	}

	if isDarwinProxyEnabled(values, "HTTPEnable") {
		proxyURL, err := buildDarwinProxyURL(values["HTTPProxy"], values["HTTPPort"], "http")
		if err != nil {
			return nil, "", err
		}
		selection.HTTPProxy = proxyURL
	}
	if isDarwinProxyEnabled(values, "HTTPSEnable") {
		proxyURL, err := buildDarwinProxyURL(values["HTTPSProxy"], values["HTTPSPort"], "http")
		if err != nil {
			return nil, "", err
		}
		selection.HTTPSProxy = proxyURL
	}
	if isDarwinProxyEnabled(values, "SOCKSEnable") {
		proxyURL, err := buildDarwinProxyURL(values["SOCKSProxy"], values["SOCKSPort"], "socks5")
		if err != nil {
			return nil, "", err
		}
		selection.AllProxy = proxyURL
	}

	if selection.HasProxy() {
		return selection, "", nil
	}

	notes := make([]string, 0, 2)
	if isDarwinProxyEnabled(values, "ProxyAutoConfigEnable") || strings.TrimSpace(values["ProxyAutoConfigURLString"]) != "" {
		notes = append(notes, "PAC detected")
	}
	if isDarwinProxyEnabled(values, "ProxyAutoDiscoveryEnable") {
		notes = append(notes, "auto discovery detected")
	}
	return nil, strings.Join(notes, ", "), nil
}

func parseDarwinProxyDictionary(raw string) (map[string]string, map[string][]string) {
	values := make(map[string]string)
	arrays := make(map[string][]string)
	currentArray := ""

	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || line == "<dictionary> {" {
			continue
		}
		if line == "}" {
			currentArray = ""
			continue
		}

		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if strings.HasPrefix(value, "<array>") {
			currentArray = key
			if _, exists := arrays[currentArray]; !exists {
				arrays[currentArray] = nil
			}
			continue
		}
		if currentArray != "" {
			if _, err := strconv.Atoi(key); err == nil {
				arrays[currentArray] = append(arrays[currentArray], value)
			}
			continue
		}
		values[key] = value
	}

	return values, arrays
}

func isDarwinProxyEnabled(values map[string]string, key string) bool {
	return strings.TrimSpace(values[key]) == "1"
}

func buildDarwinProxyURL(host string, port string, defaultScheme string) (*url.URL, error) {
	host = strings.TrimSpace(host)
	if host == "" {
		return nil, nil
	}
	port = strings.TrimSpace(port)
	if port != "" {
		if _, err := strconv.Atoi(port); err != nil {
			return nil, fmt.Errorf("invalid macOS proxy port %q: %w", port, err)
		}
		host = net.JoinHostPort(host, port)
	}
	return parseProxyURL(host, defaultScheme)
}
