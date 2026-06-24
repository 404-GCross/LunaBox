package utils

import (
	"errors"
	"fmt"
	"os"

	"github.com/joho/godotenv"
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
