package runtime

import (
	"altinn.studio/altinn-k8s-operator/internal/config"
	operatorcontext "altinn.studio/altinn-k8s-operator/internal/operator_context"
)

type Runtime interface {
	GetConfig() *config.Config
	GetOperatorContext() *operatorcontext.Context
}
