package config

import (
	"github.com/altinn/altinn-k8s-operator/internal/operatorcontext"
	"github.com/altinn/altinn-k8s-operator/internal/telemetry"
	"github.com/go-playground/validator/v10"
	"go.opentelemetry.io/otel"
)

type Config struct {
	MaskinportenApi MaskinportenApiConfig `koanf:"maskinporten_api" validate:"required"`
}

type MaskinportenApiConfig struct {
	ClientId string `koanf:"client_id" validate:"required"`
	Url      string `koanf:"url"       validate:"required,http_url"`
	Jwk      string `koanf:"jwk"       validate:"required,json"`
	Scope    string `koanf:"scope"     validate:"required"`
}

func GetConfig(operatorContext *operatorcontext.Context, configFilePath string) (*Config, error) {
	tracer := otel.Tracer(telemetry.ServiceName)
	ctx, span := tracer.Start(operatorContext, "GetConfig")
	operatorContext.Update(ctx)
	defer span.End()

	var cfg *Config
	var err error
	if operatorContext.IsLocal() {
		cfg, err = loadFromKoanf(operatorContext, configFilePath)
	} else {
		cfg, err = loadFromAzureKeyVault(operatorContext)
	}

	if err != nil {
		return nil, err
	}

	validate := validator.New(validator.WithRequiredStructEnabled())

	if err := validate.Struct(cfg); err != nil {
		return nil, err
	}

	// k.Print() // Uncomment to print the config, only for debug, there be secrets

	return cfg, nil
}

func GetConfigOrDie(operatorContext *operatorcontext.Context, configFilePath string) *Config {
	cfg, err := GetConfig(operatorContext, configFilePath)
	if err != nil {
		panic(err)
	}
	return cfg
}
