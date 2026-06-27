package linter

import (
	"context"

	"github.com/golangci/golangci-lint/v2/pkg/config"
	"github.com/golangci/golangci-lint/v2/pkg/result"
)

type Linter interface {
	Run(ctx context.Context, lintCtx *Context) ([]*result.Issue, error)
	Name() string
	Desc() string
}

type Noop struct {
	name   string
	desc   string
	reason string
	level  DeprecationLevel
}

func NewNoop(l Linter, reason string) Noop {
	return Noop{
		name:   l.Name(),
		desc:   l.Desc(),
		reason: reason,
	}
}

func NewNoopDeprecated(name string, cfg *config.Config, level DeprecationLevel) Noop {
	noop := Noop{
		name:   name,
		desc:   "Deprecated",
		reason: "This linter is fully inactivated: it will not produce any reports.",
		level:  level,
	}

	if cfg.InternalCmdTest {
		noop.reason = ""
	}

	return noop
}

func (n Noop) Run(_ context.Context, lintCtx *Context) ([]*result.Issue, error) {
	if n.reason == "" {
		return nil, nil
	}

	switch n.level {
	case DeprecationError:
		lintCtx.Log.Errorf("%s: %s", n.name, n.reason)
	default:
		lintCtx.Log.Warnf("%s: %s", n.name, n.reason)
	}

	return nil, nil
}

func (n Noop) Name() string {
	return n.name
}

func (n Noop) Desc() string {
	return n.desc
}
