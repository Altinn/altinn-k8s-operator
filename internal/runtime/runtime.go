package runtime

import (
	"github.com/altinn/altinn-k8s-operator/internal/config"
	"github.com/altinn/altinn-k8s-operator/internal/maskinporten"
	"github.com/altinn/altinn-k8s-operator/internal/operatorcontext"
)

type Runtime interface {
	GetConfig() *config.Config
	GetOperatorContext() *operatorcontext.Context
	GetMaskinportenClientManager() maskinporten.ClientManager
}
