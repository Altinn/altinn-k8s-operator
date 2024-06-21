package internal

import (
	"context"

	"github.com/altinn/altinn-k8s-operator/internal/config"
	"github.com/altinn/altinn-k8s-operator/internal/maskinporten"
	"github.com/altinn/altinn-k8s-operator/internal/operatorcontext"
	rt "github.com/altinn/altinn-k8s-operator/internal/runtime"
	"github.com/altinn/altinn-k8s-operator/internal/telemetry"
	"github.com/jonboulle/clockwork"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

type runtime struct {
	config          config.Config
	operatorContext operatorcontext.Context
	clientManager   maskinporten.ClientManager
	tracer          trace.Tracer
	meter           metric.Meter
}

var _ rt.Runtime = (*runtime)(nil)

func NewRuntime(ctx context.Context) (rt.Runtime, error) {
	tracer := otel.Tracer(telemetry.ServiceName)
	ctx, span := tracer.Start(ctx, "NewRuntime")
	defer span.End()

	operatorContext, err := operatorcontext.Discover(ctx)
	if err != nil {
		return nil, err
	}

	cfg, err := config.GetConfig(operatorContext, "")
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
		tracer:          tracer,
		meter:           otel.Meter(telemetry.ServiceName),
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

func (r *runtime) Tracer() trace.Tracer {
	return r.tracer
}

func (r *runtime) Meter() metric.Meter {
	return r.meter
}
