package config

import (
	"sync"

	"github.com/godoc-lint/godoc-lint/pkg/model"
)

// OnceConfigBuilder wraps a config builder and make it a one-time builder, so
// that further attempts to build will return the same result.
//
// This type is concurrent-safe.
type OnceConfigBuilder struct {
	builder model.ConfigBuilder

	mu sync.Mutex
	m  map[string]built
}

type built struct {
	value model.Config
	err   error
}

// NewOnceConfigBuilder crates a new instance of the corresponding struct.
func NewOnceConfigBuilder(builder model.ConfigBuilder) *OnceConfigBuilder {
	return &OnceConfigBuilder{
		builder: builder,
	}
}

// GetConfig implements the corresponding interface method.
func (ocb *OnceConfigBuilder) GetConfig(cwd string) (model.Config, error) {
	ocb.mu.Lock()
	defer ocb.mu.Unlock()

	if b, ok := ocb.m[cwd]; ok {
		return b.value, b.err
	}

	b := built{}
	b.value, b.err = ocb.builder.GetConfig(cwd)
	if ocb.m == nil {
		ocb.m = make(map[string]built, 10)
	}
	ocb.m[cwd] = b
	return b.value, b.err
}

// SetOverride implements the corresponding interface method.
func (ocb *OnceConfigBuilder) SetOverride(override *model.ConfigOverride) {
	ocb.mu.Lock()
	defer ocb.mu.Unlock()

	if len(ocb.m) > 0 {
		return
	}
	ocb.builder.SetOverride(override)
}
