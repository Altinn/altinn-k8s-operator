package operatorcontext

import (
	"context"

	"github.com/altinn/altinn-k8s-operator/internal/telemetry"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

const EnvironmentLocal = "local"

type Context struct {
	ServiceOwner string
	Environment  string
	// Context which will be cancelled when the program is shut down
	Context context.Context
	tracer  trace.Tracer
}

func (c *Context) IsLocal() bool {
	return c.Environment == EnvironmentLocal
}

func Discover(ctx context.Context) (*Context, error) {
	err := ctx.Err()
	if err != nil {
		return nil, err
	}

	// This should come from the environment/context somewhere
	// there should be 1:1 mapping between TE/env:cluster
	serviceOwner := "local"
	environment := EnvironmentLocal

	return &Context{
		ServiceOwner: serviceOwner,
		Environment:  environment,
		Context:      ctx,
		tracer:       otel.Tracer(telemetry.ServiceName),
	}, nil
}

func DiscoverOrDie(ctx context.Context) *Context {
	context, err := Discover(ctx)
	if err != nil {
		panic(err)
	}
	return context
}

func (c *Context) StartSpan(
	spanName string,
	opts ...trace.SpanStartOption,
) trace.Span {
	ctx, span := c.tracer.Start(c.Context, spanName, opts...)
	c.Context = ctx
	return span
}
