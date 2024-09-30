package config

import (
	"fmt"
	"os"
	"path/filepath"
)

func TryFindProjectRoot() string {
	for {
		if _, err := os.Stat("go.mod"); err == nil {
			currentDir, err := os.Getwd()
			if err != nil {
				return ""
			}
			return currentDir
		}

		if err := os.Chdir(".."); err != nil {
			return ""
		}
	}
}

func GetConfigFilePathForEnv(env string) string {
	rootDir := TryFindProjectRoot()
	if rootDir == "" {
		return ""
	}

	return filepath.Join(rootDir, fmt.Sprintf("%s.env", env))
}
