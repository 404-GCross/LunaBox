package service

import (
	"lunabox/internal/appconf"
	"lunabox/internal/utils/metadata"
)

func metadataProxyOption(config *appconf.AppConfig) metadata.GetterOption {
	if config == nil {
		return nil
	}
	return metadata.WithProxy(config.DownloadProxyMode, config.DownloadProxyURL)
}

func proxyConfig(config *appconf.AppConfig) (string, string) {
	if config == nil {
		return "", ""
	}
	return config.DownloadProxyMode, config.DownloadProxyURL
}
