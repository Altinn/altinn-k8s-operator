package config

import (
	"fmt"
	"os"
	"path"

	"github.com/altinn/altinn-k8s-operator/internal/operatorcontext"
	"github.com/altinn/altinn-k8s-operator/internal/telemetry"
	"github.com/knadh/koanf/parsers/dotenv"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
	"go.opentelemetry.io/otel"
)

var (
	k      = koanf.New(".")
	parser = dotenv.ParserEnv("", ".", func(s string) string { return s })
)

func loadFromKoanf(operatorContext *operatorcontext.Context, configFilePath string) (*Config, error) {
	tracer := otel.Tracer(telemetry.ServiceName)
	ctx, span := tracer.Start(operatorContext, "GetConfig.Koanf")
	operatorContext.Update(ctx)
	defer span.End()

	tryFindProjectRoot()

	if configFilePath == "" {
		configFilePath = "local.env"
	}

	if !operatorContext.IsLocal() {
		return nil, fmt.Errorf("loading config from koanf is only supported for local environment")
	}

	currentDir, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(configFilePath); os.IsNotExist(err) {
		if path.IsAbs(configFilePath) {
			return nil, fmt.Errorf("env file does not exist: '%s'", configFilePath)
		} else {
			return nil, fmt.Errorf("env file does not exist in '%s': '%s'", currentDir, configFilePath)
		}
	}

	if !path.IsAbs(configFilePath) {
		configFilePath = path.Join(currentDir, configFilePath)
	}

	if err := k.Load(file.Provider(configFilePath), parser); err != nil {
		return nil, fmt.Errorf("error loading config '%s': %w", configFilePath, err)
	}

	var cfg Config

	if err := k.Unmarshal("", &cfg); err != nil {
		return nil, fmt.Errorf("error unmarshalling config '%s': %w", configFilePath, err)
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
