package rules

import (
	"bufio"
	"errors"
	"fmt"
	"go/types"
	"io"
	"strings"

	"golang.org/x/tools/go/types/typeutil"
)

var ErrInvalidRule = errors.New("invalid rule format")

const CustomRulesetName = "custom"

type Ruleset struct {
	Name          string
	PackageImport string
	Rules         []FuncRule

	ruleIndicesByFuncName map[string][]int
}

func (rs *Ruleset) Match(fn *types.Func) bool {
	// PackageImport is already checked (by indices), skip checking it here
	sig := fn.Type().(*types.Signature) // it's safe since we already checked

	// Fail fast if the function name is not in the rule list.
	indices, ok := rs.ruleIndicesByFuncName[fn.Name()]
	if !ok {
		return false
	}

	for _, idx := range indices {
		rule := &rs.Rules[idx]
		if matchRule(rule, sig) {
			return true
		}
	}

	return false
}

var receiverTypeCache = typeutil.Map{}

func receiverTypeOf(recvType types.Type) (repr string) {
	if val := receiverTypeCache.At(recvType); val != nil {
		repr, _ = val.(string)
		return repr
	}

	defer func() {
		receiverTypeCache.Set(recvType, repr)
	}()

	buf := &strings.Builder{}
	var recvNamed *types.Named
	switch recvType := recvType.(type) {
	case *types.Pointer:
		buf.WriteByte('*')
		if elem, ok := recvType.Elem().(*types.Named); ok {
			recvNamed = elem
		}
	case *types.Named:
		recvNamed = recvType
	}

	if recvNamed == nil {
		// not supported type
		return ""
	}

	buf.WriteString(recvNamed.Obj().Name())
	typeParams := recvNamed.TypeParams()
	if typeParamsLen := typeParams.Len(); typeParamsLen > 0 {
		buf.WriteByte('[')
		for i := 0; i < typeParamsLen; i++ {
			if i > 0 {
				// comma as separator
				buf.WriteByte(',')
			}
			p := typeParams.At(i)
			buf.WriteString(p.Obj().Name())
		}
		buf.WriteByte(']')
	}

	return buf.String()
}

func matchRule(p *FuncRule, sig *types.Signature) bool {
	// we do not check package import here since it's already checked in Match()
	recv := sig.Recv()
	isReceiver := recv != nil
	if isReceiver != p.IsReceiver {
		return false
	}

	if isReceiver {
		recvType := recv.Type()
		receiverType := receiverTypeOf(recvType)
		if receiverType != p.ReceiverType {
			return false
		}
	}

	return true
}

type FuncRule struct { // package import should be accessed from Rulset
	ReceiverType string
	FuncName     string
	IsReceiver   bool
}

func ParseFuncRule(rule string) (packageImport string, pat FuncRule, err error) {
	lastDot := strings.LastIndexFunc(rule, func(r rune) bool {
		return r == '.' || r == '/'
	})
	if lastDot == -1 || rule[lastDot] == '/' {
		return "", pat, ErrInvalidRule
	}

	importOrReceiver := rule[:lastDot]
	pat.FuncName = rule[lastDot+1:]

	if strings.HasPrefix(rule, "(") { // package
		if !strings.HasSuffix(importOrReceiver, ")") {
			return "", FuncRule{}, ErrInvalidRule
		}

		var isPointerReceiver bool
		pat.IsReceiver = true
		receiver := importOrReceiver[1 : len(importOrReceiver)-1]
		if strings.HasPrefix(receiver, "*") {
			isPointerReceiver = true
			receiver = receiver[1:]
		}

		typeDotIdx := strings.LastIndexFunc(receiver, func(r rune) bool {
			return r == '.' || r == '/'
		})
		if typeDotIdx == -1 || receiver[typeDotIdx] == '/' {
			return "", FuncRule{}, ErrInvalidRule
		}
		receiverType := receiver[typeDotIdx+1:]
		if isPointerReceiver {
			receiverType = "*" + receiverType
		}
		pat.ReceiverType = receiverType
		packageImport = receiver[:typeDotIdx]
	} else {
		packageImport = importOrReceiver
	}

	return packageImport, pat, nil
}

func ParseRules(lines []string) (result []Ruleset, err error) {
	rulesByImport := make(map[string][]FuncRule)
	for i, line := range lines {
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "#") { // comments
			continue
		}

		packageImport, pat, err := ParseFuncRule(line)
		if err != nil {
			return nil, fmt.Errorf("error parse rule at line %d: %w", i+1, err)
		}
		rulesByImport[packageImport] = append(rulesByImport[packageImport], pat)
	}

	for packageImport, rules := range rulesByImport {
		ruleIndicesByFuncName := make(map[string][]int, len(rules))
		for idx, rule := range rules {
			fnName := rule.FuncName
			ruleIndicesByFuncName[fnName] = append(ruleIndicesByFuncName[fnName], idx)
		}

		result = append(result, Ruleset{
			Name:                  CustomRulesetName, // NOTE(timonwong) Always "custom" for custom rule
			PackageImport:         packageImport,
			Rules:                 rules,
			ruleIndicesByFuncName: ruleIndicesByFuncName,
		})
	}
	return result, nil
}

func ParseRuleFile(r io.Reader) (result []Ruleset, err error) {
	// Rule files are relatively small, so read it into string slice first.
	var lines []string

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		lines = append(lines, line)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return ParseRules(lines)
}
