package p

import "go/ast"

func test(s ast.Stmt) {
	switch s := s.(type) {
	}
}
