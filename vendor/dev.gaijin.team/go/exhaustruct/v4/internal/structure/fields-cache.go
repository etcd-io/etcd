package structure

import (
	"go/types"
	"sync"
)

type FieldsCache struct {
	fields map[*types.Struct]Fields
	mu     sync.RWMutex
}

// Get returns a struct fields for a given type. In case if a struct fields is
// not found, it creates a new one from type definition.
func (c *FieldsCache) Get(typ *types.Struct) Fields {
	c.mu.RLock()
	fields, ok := c.fields[typ]
	c.mu.RUnlock()

	if ok {
		return fields
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.fields == nil {
		c.fields = make(map[*types.Struct]Fields)
	}

	fields = NewFields(typ)
	c.fields[typ] = fields

	return fields
}
