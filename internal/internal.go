package internal

import (
	"altinn.studio/altinn-k8s-operator/internal/config"
	operatorcontext "altinn.studio/altinn-k8s-operator/internal/operator_context"
	rt "altinn.studio/altinn-k8s-operator/internal/runtime"
)

type runtime struct {
	config          config.Config
	operatorContext operatorcontext.Context
}

var _ rt.Runtime = (*runtime)(nil)

func NewRuntime() (rt.Runtime, error) {
	operatorContext, err := operatorcontext.Discover()
	if err != nil {
		return nil, err
	}

	cfg, err := config.GetConfig(operatorContext)
	if err != nil {
		return nil, err
	}

	rt := &runtime{
		config:          *cfg,
		operatorContext: *operatorContext,
	}

	return rt, nil
}

func (r *runtime) GetConfig() *config.Config {
	return &r.config
}

func (r *runtime) GetOperatorContext() *operatorcontext.Context {
	return &r.operatorContext
}
