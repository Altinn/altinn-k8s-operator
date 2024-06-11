package internal

import (
	"altinn.studio/altinn-k8s-operator/internal/config"
	"altinn.studio/altinn-k8s-operator/internal/maskinporten"
	operatorcontext "altinn.studio/altinn-k8s-operator/internal/operator_context"
	rt "altinn.studio/altinn-k8s-operator/internal/runtime"
	"github.com/jonboulle/clockwork"
)

type runtime struct {
	config          config.Config
	operatorContext operatorcontext.Context
	clientManager   maskinporten.ClientManager
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

	clock := clockwork.NewRealClock()
	clientManager, err := maskinporten.NewClientManager(&cfg.MaskinportenApi, clock)
	if err != nil {
		return nil, err
	}

	rt := &runtime{
		config:          *cfg,
		operatorContext: *operatorContext,
		clientManager:   clientManager,
	}

	return rt, nil
}

func (r *runtime) GetConfig() *config.Config {
	return &r.config
}

func (r *runtime) GetOperatorContext() *operatorcontext.Context {
	return &r.operatorContext
}

func (r *runtime) GetMaskinportenClientManager() maskinporten.ClientManager {
	return r.clientManager
}
