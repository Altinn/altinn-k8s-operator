package operatorcontext

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/trace"
)

const EnvLocal = "local"

var _ context.Context = (*Context)(nil)

type Context struct {
	Te    string
	Env   string
	inner context.Context
}

func (c *Context) IsLocal() bool {
	return c.Env == EnvLocal
}

func Discover(ctx context.Context) (*Context, error) {
	err := ctx.Err()
	if err != nil {
		return nil, err
	}

	// This should come from the environment/context somewhere
	// there should be 1:1 mapping between TE/env:cluster
	te := "local"
	env := EnvLocal

	return &Context{Te: te, Env: env, inner: ctx}, nil
}

func DiscoverOrDie(ctx context.Context) *Context {
	context, err := Discover(ctx)
	if err != nil {
		panic(err)
	}
	return context
}

func (c *Context) Deadline() (deadline time.Time, ok bool) {
	return c.inner.Deadline()
}

func (c *Context) Done() <-chan struct{} {
	return c.inner.Done()
}

func (c *Context) Err() error {
	return c.inner.Err()
}

func (c *Context) Value(key any) any {
	return c.inner.Value(key)
}

func (c *Context) Update(ctx context.Context) {
	c.inner = ctx
}

func (c *Context) ClearCurrentSpan() {
	c.inner = trace.ContextWithSpan(c.inner, nil)
}
