package ginkgoinfo

import (
	gotypes "go/types"
	"strings"
)

const (
	ctxTypeName     = "context.Context"
	ginkgoCtxSuffix = "github.com/onsi/ginkgo/v2/internal.SpecContext"
)

func IsGinkgoContext(t gotypes.Type) bool {
	maybeCtx := gotypes.Unalias(t)

	typeName := maybeCtx.String()
	if typeName == ctxTypeName {
		return true
	}

	if strings.HasSuffix(typeName, ginkgoCtxSuffix) {
		return true
	}

	return false
}
