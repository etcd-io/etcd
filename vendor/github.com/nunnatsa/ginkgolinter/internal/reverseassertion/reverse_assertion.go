package reverseassertion

import "go/token"

var reverseLogicAssertions = map[string]string{
	"To":        "ToNot",
	"ToNot":     "To",
	"NotTo":     "To",
	"Should":    "ShouldNot",
	"ShouldNot": "Should",
}

// ChangeAssertionLogic get gomega assertion function name, and returns the reverse logic function name
func ChangeAssertionLogic(funcName string) string {
	if revFunc, ok := reverseLogicAssertions[funcName]; ok {
		return revFunc
	}
	return funcName
}

func IsNegativeLogic(funcName string) bool {
	switch funcName {
	case "ToNot", "NotTo", "ShouldNot":
		return true
	}
	return false
}

var reverseCompareOperators = map[token.Token]token.Token{
	token.LSS: token.GTR,
	token.GTR: token.LSS,
	token.LEQ: token.GEQ,
	token.GEQ: token.LEQ,
}

// ChangeCompareOperator return the reversed comparison operator
func ChangeCompareOperator(op token.Token) token.Token {
	if revOp, ok := reverseCompareOperators[op]; ok {
		return revOp
	}
	return op
}
