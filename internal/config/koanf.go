package config

import (
	"fmt"
	"os"
	"path"

	operatorcontext "altinn.studio/altinn-k8s-operator/internal/operator_context"
	"github.com/knadh/koanf/parsers/dotenv"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

var (
	k      = koanf.New(".")
	parser = dotenv.ParserEnv("", ".", func(s string) string { return s })
)

func loadFromKoanf(operatorContext *operatorcontext.Context) (*Config, error) {
	tryFindProjectRoot()

	filePath := "local.env"
	if !operatorContext.IsLocal() {
		return nil, fmt.Errorf("loading config from koanf is only supported for local environment")
	}

	currentDir, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		if path.IsAbs(filePath) {
			return nil, fmt.Errorf("env file does not exist: '%s'", filePath)
		} else {
			return nil, fmt.Errorf("env file does not exist in '%s': '%s'", currentDir, filePath)
		}
	}

	if !path.IsAbs(filePath) {
		filePath = path.Join(currentDir, filePath)
	}

	if err := k.Load(file.Provider(filePath), parser); err != nil {
		return nil, fmt.Errorf("error loading config '%s': %w", filePath, err)
	}

	var cfg Config

	if err := k.Unmarshal("", &cfg); err != nil {
		return nil, fmt.Errorf("error unmarshalling config '%s': %w", filePath, err)
	}

	return &cfg, nil
}

func tryFindProjectRoot() {
	for {
		if _, err := os.Stat("go.mod"); err == nil {
			return
		}

		if err := os.Chdir(".."); err != nil {
			return
		}
	}
}
