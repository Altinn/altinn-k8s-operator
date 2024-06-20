package operatorcontext

const EnvLocal = "local"

type Context struct {
	Te  string
	Env string
}

func (c *Context) IsLocal() bool {
	return c.Env == EnvLocal
}

func Discover() (*Context, error) {
	// This should come from the environment/context somewhere
	// there should be 1:1 mapping between TE/env:cluster
	te := "local"
	env := EnvLocal

	return &Context{Te: te, Env: env}, nil
}

func DiscoverOrDie() *Context {
	context, err := Discover()
	if err != nil {
		panic(err)
	}
	return context
}
