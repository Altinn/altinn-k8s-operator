package runtime

import (
	"github.com/altinn/altinn-k8s-operator/internal/config"
	"github.com/altinn/altinn-k8s-operator/internal/crypto"
	"github.com/altinn/altinn-k8s-operator/internal/maskinporten"
	"github.com/altinn/altinn-k8s-operator/internal/operatorcontext"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

type Runtime interface {
	GetConfig() *config.Config
	GetOperatorContext() *operatorcontext.Context
	GetCrypto() *crypto.CryptoService
	GetMaskinportenApiClient() *maskinporten.HttpApiClient
	Tracer() trace.Tracer
	Meter() metric.Meter
}
