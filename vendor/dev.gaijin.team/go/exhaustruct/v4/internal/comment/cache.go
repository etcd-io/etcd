package comment

import (
	"go/ast"
	"go/token"
	"sync"
)

type Cache struct {
	comments map[*ast.File]ast.CommentMap
	mu       sync.RWMutex
}

// Get returns a comment map for a given file. In case if a comment map is not
// found, it creates a new one.
func (c *Cache) Get(fset *token.FileSet, f *ast.File) ast.CommentMap {
	c.mu.RLock()
	if cm, ok := c.comments[f]; ok {
		c.mu.RUnlock()
		return cm
	}
	c.mu.RUnlock()

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.comments == nil {
		c.comments = make(map[*ast.File]ast.CommentMap)
	}

	cm := ast.NewCommentMap(fset, f, f.Comments)
	c.comments[f] = cm

	return cm
}
