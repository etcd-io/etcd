package actual

import (
	gotypes "go/types"

	"github.com/nunnatsa/ginkgolinter/internal/gomegainfo"
	"github.com/nunnatsa/ginkgolinter/internal/typecheck"
)

func getAsyncFuncArg(sig *gotypes.Signature) ArgPayload {
	argType := FuncSigArgType
	if sig.Results().Len() == 1 {
		if typecheck.ImplementsError(sig.Results().At(0).Type().Underlying()) {
			argType |= ErrFuncActualArgType | ErrorTypeArgType
		}
	}

	if sig.Params().Len() > 0 {
		arg := sig.Params().At(0).Type()
		if sig.Results().Len() == 0 {
			if gomegainfo.IsGomegaType(arg) {
				argType |= FuncSigArgType | GomegaParamArgType
			}
			if typecheck.ImplementsTB(arg) {
				argType |= FuncSigArgType | TBParamArgType
			}
		}
	}

	if sig.Results().Len() > 1 {
		argType |= FuncSigArgType | MultiRetsArgType
	}

	return &FuncSigArgPayload{argType: argType}
}

type FuncSigArgPayload struct {
	argType ArgType
}

func (f FuncSigArgPayload) ArgType() ArgType {
	return f.argType
}
