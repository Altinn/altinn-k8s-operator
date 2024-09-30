package fakes

type State struct {
	Db Db
}

func NewState() *State {
	return &State{
		Db: NewDb(),
	}
}
