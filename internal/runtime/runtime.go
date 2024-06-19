package runtime

import (
	"github.com/altinn/altinn-k8s-operator/internal/config"
	"github.com/altinn/altinn-k8s-operator/internal/maskinporten"
	operatorcontext "github.com/altinn/altinn-k8s-operator/internal/operator_context"
)

type Runtime interface {
	GetConfig() *config.Config
	GetOperatorContext() *operatorcontext.Context
	GetMaskinportenClientManager() maskinporten.ClientManager
}
