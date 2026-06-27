package typecheck

import (
	"go/token"
	gotypes "go/types"
)

var (
	errorType         *gotypes.Interface
	gomegaMatcherType *gotypes.Interface
	tbTypes           = []*gotypes.Interface{
		// In practice, interfaces which mimick testing.TB probably implement
		// more than one of these at the same time. But for ImplementsTB
		// it's sufficient to have just one method which can be used
		// to report a test failure.
		tbInterface("Error", false),
		tbInterface("Errorf", true),
		tbInterface("Fatal", false),
		tbInterface("Fatalf", true),
	}
)

func init() {
	errorType = gotypes.Universe.Lookup("error").Type().Underlying().(*gotypes.Interface)
	gomegaMatcherType = generateTheGomegaMatcherInfType()
}

// generateTheGomegaMatcherInfType generates a types.Interface instance that represents the
// GomegaMatcher interface.
// The original code is (copied from https://github.com/nunnatsa/ginkgolinter/blob/8fdd05eee922578d4699f49d267001c01e0b9f1e/testdata/src/a/vendor/github.com/onsi/gomega/types/types.go)
//
//	type GomegaMatcher interface {
//		Match(actual interface{}) (success bool, err error)
//		FailureMessage(actual interface{}) (message string)
//		NegatedFailureMessage(actual interface{}) (message string)
//	}
func generateTheGomegaMatcherInfType() *gotypes.Interface {
	err := gotypes.Universe.Lookup("error").Type()
	bl := gotypes.Typ[gotypes.Bool]
	str := gotypes.Typ[gotypes.String]
	anyType := gotypes.Universe.Lookup("any").Type()

	return gotypes.NewInterfaceType([]*gotypes.Func{
		// Match(actual interface{}) (success bool, err error)
		gotypes.NewFunc(token.NoPos, nil, "Match", gotypes.NewSignatureType(
			nil, nil, nil,
			gotypes.NewTuple(
				gotypes.NewVar(token.NoPos, nil, "actual", anyType),
			),
			gotypes.NewTuple(
				gotypes.NewVar(token.NoPos, nil, "", bl),
				gotypes.NewVar(token.NoPos, nil, "", err),
			), false),
		),
		// FailureMessage(actual interface{}) (message string)
		gotypes.NewFunc(token.NoPos, nil, "FailureMessage", gotypes.NewSignatureType(
			nil, nil, nil,
			gotypes.NewTuple(
				gotypes.NewVar(token.NoPos, nil, "", anyType),
			),
			gotypes.NewTuple(
				gotypes.NewVar(token.NoPos, nil, "", str),
			),
			false),
		),
		//NegatedFailureMessage(actual interface{}) (message string)
		gotypes.NewFunc(token.NoPos, nil, "NegatedFailureMessage", gotypes.NewSignatureType(
			nil, nil, nil,
			gotypes.NewTuple(
				gotypes.NewVar(token.NoPos, nil, "", anyType),
			),
			gotypes.NewTuple(
				gotypes.NewVar(token.NoPos, nil, "", str),
			),
			false),
		),
	}, nil)
}

// ImplementsError checks if a type implements the error interface
func ImplementsError(t gotypes.Type) bool {
	return gotypes.Implements(t, errorType)
}

// ImplementsGomegaMatcher checks if a type implements the GomegaMatcher interface
func ImplementsGomegaMatcher(t gotypes.Type) bool {
	return t != nil && gotypes.Implements(t, gomegaMatcherType)
}

// ImplementsTB checks if the argument type implements any of the methods in testing.TB which
// can be used to report test failures. Such a type is a potential alternative to a Gomega
// parameter in some Gomega wrappers.
func ImplementsTB(t gotypes.Type) bool {
	for _, tbType := range tbTypes {
		if gotypes.Implements(t, tbType) {
			return true
		}
	}
	return false
}

// tbInterface generates an interface type with exactly one method
// which has the given name and Printf or Println signature.
func tbInterface(name string, printf bool) *gotypes.Interface {
	var params []*gotypes.Var
	if printf {
		params = append(params, gotypes.NewVar(0, nil, "", gotypes.Typ[gotypes.String]))
	}
	params = append(params, gotypes.NewVar(0, nil, "", gotypes.NewSlice(gotypes.Universe.Lookup("any").Type())))
	signature := gotypes.NewSignatureType(nil, nil, nil,
		gotypes.NewTuple(params...),
		gotypes.NewTuple(),
		true,
	)
	method := gotypes.NewFunc(0, nil, name, signature)
	return gotypes.NewInterfaceType([]*gotypes.Func{method}, nil)
}
