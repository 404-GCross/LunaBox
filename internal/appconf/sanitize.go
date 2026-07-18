package appconf

import "strings"

func SanitizeUmbraConfig(config *AppConfig) bool {
	if config == nil {
		return false
	}

	baseURL := strings.TrimRight(strings.TrimSpace(config.UmbraBaseURL), "/")
	changed := config.UmbraBaseURL != baseURL
	if strings.EqualFold(baseURL, legacyUmbraStageBaseURL) {
		baseURL = DefaultUmbraBaseURL
		config.UmbraAuthenticated = false
		changed = true
	}
	config.UmbraBaseURL = baseURL
	if baseURL == "" && config.UmbraAuthenticated {
		config.UmbraAuthenticated = false
		changed = true
	}
	return changed
}

func SanitizeOneDriveOAuthConfig(config *AppConfig) bool {
	if config == nil {
		return false
	}

	trimmedClientID := strings.TrimSpace(config.OneDriveClientID)
	changed := config.OneDriveClientID != trimmedClientID
	config.OneDriveClientID = trimmedClientID

	if config.OneDriveClientID == legacyOneDriveDefaultClientID {
		config.OneDriveClientID = ""
		changed = true
		if config.OneDriveRefreshToken != "" {
			config.OneDriveRefreshToken = ""
		}
	}

	return changed
}

func SanitizeBangumiOAuthConfig(config *AppConfig) bool {
	if config == nil {
		return false
	}

	trimmedAccessToken := strings.TrimSpace(config.BangumiAccessToken)
	trimmedRefreshToken := strings.TrimSpace(config.BangumiRefreshToken)
	trimmedExpiresAt := strings.TrimSpace(config.BangumiTokenExpiresAt)
	trimmedUserID := strings.TrimSpace(config.BangumiAuthorizedUserID)
	trimmedUsername := strings.TrimSpace(config.BangumiAuthorizedUsername)
	trimmedAvatarURL := strings.TrimSpace(config.BangumiAuthorizedAvatarURL)
	trimmedAuthError := strings.TrimSpace(config.BangumiAuthError)

	changed := config.BangumiAccessToken != trimmedAccessToken ||
		config.BangumiRefreshToken != trimmedRefreshToken ||
		config.BangumiTokenExpiresAt != trimmedExpiresAt ||
		config.BangumiAuthorizedUserID != trimmedUserID ||
		config.BangumiAuthorizedUsername != trimmedUsername ||
		config.BangumiAuthorizedAvatarURL != trimmedAvatarURL ||
		config.BangumiAuthError != trimmedAuthError

	config.BangumiAccessToken = trimmedAccessToken
	config.BangumiRefreshToken = trimmedRefreshToken
	config.BangumiTokenExpiresAt = trimmedExpiresAt
	config.BangumiAuthorizedUserID = trimmedUserID
	config.BangumiAuthorizedUsername = trimmedUsername
	config.BangumiAuthorizedAvatarURL = trimmedAvatarURL
	config.BangumiAuthError = trimmedAuthError
	if config.BangumiStatusPushEnabled == nil {
		config.BangumiStatusPushEnabled = boolPtr(true)
		changed = true
	}

	if config.BangumiAccessToken == "" && config.BangumiTokenExpiresAt != "" {
		config.BangumiTokenExpiresAt = ""
		changed = true
	}

	return changed
}
