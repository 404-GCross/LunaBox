package utils

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/joho/godotenv"
	"lunabox/internal/version"
)

const (
	bangumiClientIDEnv        = "LUNABOX_BANGUMI_CLIENT_ID"
	bangumiClientSecretEnv    = "LUNABOX_BANGUMI_CLIENT_SECRET"
	touchGalTokenEnv          = "LUNABOX_TOUCHGAL_TOKEN"
	umbraClientIDEnv          = "LUNABOX_UMBRA_CLIENT_ID"
	umbraRegistrationTokenEnv = "LUNABOX_UMBRA_REGISTRATION_TOKEN"
)

func LoadEnvFilesIfExists(filenames ...string) error {
	existingFiles := make([]string, 0, len(filenames))
	for _, filename := range filenames {
		if _, err := os.Stat(filename); err == nil {
			existingFiles = append(existingFiles, filename)
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("检查 env 文件 %s 失败: %w", filename, err)
		}
	}
	if len(existingFiles) == 0 {
		return nil
	}
	if err := godotenv.Load(existingFiles...); err != nil {
		return fmt.Errorf("加载 env 文件失败: %w", err)
	}
	return nil
}

// ApplyDevBuildEnvFallbacks makes build-time credentials available to wails dev.
// Real build-time ldflags keep priority; environment variables only fill blanks.
func ApplyDevBuildEnvFallbacks() {
	if strings.TrimSpace(version.BangumiOAuthClientID) == "" {
		version.BangumiOAuthClientID = strings.TrimSpace(os.Getenv(bangumiClientIDEnv))
	}
	if strings.TrimSpace(version.BangumiOAuthClientSecret) == "" {
		version.BangumiOAuthClientSecret = strings.TrimSpace(os.Getenv(bangumiClientSecretEnv))
	}
	if strings.TrimSpace(version.TouchGalAPIToken) == "" {
		version.TouchGalAPIToken = strings.TrimSpace(os.Getenv(touchGalTokenEnv))
	}
	if strings.TrimSpace(version.UmbraOAuthClientID) == "" {
		version.UmbraOAuthClientID = strings.TrimSpace(os.Getenv(umbraClientIDEnv))
	}
	if strings.TrimSpace(version.UmbraRegistrationToken) == "" {
		version.UmbraRegistrationToken = strings.TrimSpace(os.Getenv(umbraRegistrationTokenEnv))
	}
}
