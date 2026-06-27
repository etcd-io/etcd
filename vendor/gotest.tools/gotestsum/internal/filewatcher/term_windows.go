package filewatcher

import "context"

type terminal struct{}

func newTerminal() *terminal {
	return nil
}

func (r *terminal) Monitor(context.Context) {}

func (r *terminal) Events() <-chan Event {
	return nil
}

func (r *terminal) Start() {}

func (r *terminal) Reset() {}
