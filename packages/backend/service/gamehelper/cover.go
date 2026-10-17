package gamehelper

import (
	"fmt"
	"strings"
)

// MaxClipboardCoverImageBytes caps cover images accepted from the clipboard.
const MaxClipboardCoverImageBytes = 25 * 1024 * 1024

// IsDownloadableCoverURL reports whether the URL is a remote http(s) URL we should fetch.
// Wails localhost URLs are rejected since they already point at locally hosted assets.
func IsDownloadableCoverURL(coverURL string) bool {
	coverURL = strings.TrimSpace(coverURL)
	normalizedURL := strings.ToLower(coverURL)
	if coverURL == "" || strings.Contains(normalizedURL, "wails.localhost") {
		return false
	}
	return strings.HasPrefix(normalizedURL, "http://") || strings.HasPrefix(normalizedURL, "https://")
}

// SplitImageDataURL extracts the content-type and base64 payload from an image data URL.
func SplitImageDataURL(dataURL string) (string, string, error) {
	meta, encodedData, ok := strings.Cut(strings.TrimSpace(dataURL), ",")
	if !ok || !strings.HasPrefix(meta, "data:") {
		return "", "", fmt.Errorf("invalid image data URL")
	}

	metaParts := strings.Split(strings.TrimPrefix(meta, "data:"), ";")
	contentType := strings.ToLower(strings.TrimSpace(metaParts[0]))
	if !strings.HasPrefix(contentType, "image/") {
		return "", "", fmt.Errorf("unsupported cover image type: %s", contentType)
	}

	isBase64 := false
	for _, part := range metaParts[1:] {
		if strings.EqualFold(strings.TrimSpace(part), "base64") {
			isBase64 = true
			break
		}
	}
	if !isBase64 {
		return "", "", fmt.Errorf("image data URL must be base64 encoded")
	}

	encodedData = strings.TrimSpace(encodedData)
	if encodedData == "" {
		return "", "", fmt.Errorf("cover image data is empty")
	}

	return contentType, encodedData, nil
}
