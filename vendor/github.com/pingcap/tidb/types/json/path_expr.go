// Copyright 2017 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package json

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/pingcap/errors"
)

/*
	From MySQL 5.7, JSON path expression grammar:
		pathExpression ::= scope (pathLeg)*
		scope ::= [ columnReference ] '$'
		columnReference ::= // omit...
		pathLeg ::= member | arrayLocation | '**'
		member ::= '.' (keyName | '*')
		arrayLocation ::= '[' (non-negative-integer | '*') ']'
		keyName ::= ECMAScript-identifier | ECMAScript-string-literal

	And some implementation limits in MySQL 5.7:
		1) columnReference in scope must be empty now;
		2) double asterisk(**) could not be last leg;

	Examples:
		select json_extract('{"a": "b", "c": [1, "2"]}', '$.a') -> "b"
		select json_extract('{"a": "b", "c": [1, "2"]}', '$.c') -> [1, "2"]
		select json_extract('{"a": "b", "c": [1, "2"]}', '$.a', '$.c') -> ["b", [1, "2"]]
		select json_extract('{"a": "b", "c": [1, "2"]}', '$.c[0]') -> 1
		select json_extract('{"a": "b", "c": [1, "2"]}', '$.c[2]') -> NULL
		select json_extract('{"a": "b", "c": [1, "2"]}', '$.c[*]') -> [1, "2"]
		select json_extract('{"a": "b", "c": [1, "2"]}', '$.*') -> ["b", [1, "2"]]
*/

// [a-zA-Z_][a-zA-Z0-9_]* matches any identifier;
// "[^"\\]*(\\.[^"\\]*)*" matches any string literal which can carry escaped quotes;
var jsonPathExprLegRe = regexp.MustCompile(`(\.\s*([a-zA-Z_][a-zA-Z0-9_]*|\*|"[^"\\]*(\\.[^"\\]*)*")|(\[\s*([0-9]+|\*)\s*\])|\*\*)`)

type pathLegType byte

const (
	// pathLegKey indicates the path leg with '.key'.
	pathLegKey pathLegType = 0x01
	// pathLegIndex indicates the path leg with form '[number]'.
	pathLegIndex pathLegType = 0x02
	// pathLegDoubleAsterisk indicates the path leg with form '**'.
	pathLegDoubleAsterisk pathLegType = 0x03
)

// pathLeg is only used by PathExpression.
type pathLeg struct {
	typ        pathLegType
	arrayIndex int    // if typ is pathLegIndex, the value should be parsed into here.
	dotKey     string // if typ is pathLegKey, the key should be parsed into here.
}

// arrayIndexAsterisk is for parsing `*` into a number.
// we need this number represent "all".
const arrayIndexAsterisk = -1

// pathExpressionFlag holds attributes of PathExpression
type pathExpressionFlag byte

const (
	pathExpressionContainsAsterisk       pathExpressionFlag = 0x01
	pathExpressionContainsDoubleAsterisk pathExpressionFlag = 0x02
)

// containsAnyAsterisk returns true if pef contains any asterisk.
func (pef pathExpressionFlag) containsAnyAsterisk() bool {
	pef &= pathExpressionContainsAsterisk | pathExpressionContainsDoubleAsterisk
	return byte(pef) != 0
}

// PathExpression is for JSON path expression.
type PathExpression struct {
	legs  []pathLeg
	flags pathExpressionFlag
}

// popOneLeg returns a pathLeg, and a child PathExpression without that leg.
func (pe PathExpression) popOneLeg() (pathLeg, PathExpression) {
	newPe := PathExpression{
		legs:  pe.legs[1:],
		flags: 0,
	}
	for _, leg := range newPe.legs {
		if leg.typ == pathLegIndex && leg.arrayIndex == -1 {
			newPe.flags |= pathExpressionContainsAsterisk
		} else if leg.typ == pathLegKey && leg.dotKey == "*" {
			newPe.flags |= pathExpressionContainsAsterisk
		} else if leg.typ == pathLegDoubleAsterisk {
			newPe.flags |= pathExpressionContainsDoubleAsterisk
		}
	}
	return pe.legs[0], newPe
}

// popOneLastLeg returns the a parent PathExpression and the last pathLeg
func (pe PathExpression) popOneLastLeg() (PathExpression, pathLeg) {
	lastLegIdx := len(pe.legs) - 1
	lastLeg := pe.legs[lastLegIdx]
	// It is used only in modification, it has been checked that there is no asterisks.
	return PathExpression{legs: pe.legs[:lastLegIdx]}, lastLeg
}

// ContainsAnyAsterisk returns true if pe contains any asterisk.
func (pe PathExpression) ContainsAnyAsterisk() bool {
	return pe.flags.containsAnyAsterisk()
}

// ParseJSONPathExpr parses a JSON path expression. Returns a PathExpression
// object which can be used in JSON_EXTRACT, JSON_SET and so on.
func ParseJSONPathExpr(pathExpr string) (pe PathExpression, err error) {
	// Find the position of first '$'. If any no-blank characters in
	// pathExpr[0: dollarIndex), return an ErrInvalidJSONPath error.
	dollarIndex := strings.Index(pathExpr, "$")
	if dollarIndex < 0 {
		err = ErrInvalidJSONPath.GenWithStackByArgs(pathExpr)
		return
	}
	for i := 0; i < dollarIndex; i++ {
		if !isBlank(rune(pathExpr[i])) {
			err = ErrInvalidJSONPath.GenWithStackByArgs(pathExpr)
			return
		}
	}

	pathExprSuffix := strings.TrimFunc(pathExpr[dollarIndex+1:], isBlank)
	indices := jsonPathExprLegRe.FindAllStringIndex(pathExprSuffix, -1)
	if len(indices) == 0 && len(pathExprSuffix) != 0 {
		err = ErrInvalidJSONPath.GenWithStackByArgs(pathExpr)
		return
	}

	pe.legs = make([]pathLeg, 0, len(indices))
	pe.flags = pathExpressionFlag(0)

	lastEnd := 0
	for _, indice := range indices {
		start, end := indice[0], indice[1]

		// Check all characters between two legs are blank.
		for i := lastEnd; i < start; i++ {
			if !isBlank(rune(pathExprSuffix[i])) {
				err = ErrInvalidJSONPath.GenWithStackByArgs(pathExpr)
				return
			}
		}
		lastEnd = end

		if pathExprSuffix[start] == '[' {
			// The leg is an index of a JSON array.
			var leg = strings.TrimFunc(pathExprSuffix[start+1:end], isBlank)
			var indexStr = strings.TrimFunc(leg[0:len(leg)-1], isBlank)
			var index int
			if len(indexStr) == 1 && indexStr[0] == '*' {
				pe.flags |= pathExpressionContainsAsterisk
				index = arrayIndexAsterisk
			} else {
				if index, err = strconv.Atoi(indexStr); err != nil {
					err = errors.Trace(err)
					return
				}
			}
			pe.legs = append(pe.legs, pathLeg{typ: pathLegIndex, arrayIndex: index})
		} else if pathExprSuffix[start] == '.' {
			// The leg is a key of a JSON object.
			var key = strings.TrimFunc(pathExprSuffix[start+1:end], isBlank)
			if len(key) == 1 && key[0] == '*' {
				pe.flags |= pathExpressionContainsAsterisk
			} else if key[0] == '"' {
				// We need unquote the origin string.
				if key, err = unquoteString(key[1 : len(key)-1]); err != nil {
					err = ErrInvalidJSONPath.GenWithStackByArgs(pathExpr)
					return
				}
			}
			pe.legs = append(pe.legs, pathLeg{typ: pathLegKey, dotKey: key})
		} else {
			// The leg is '**'.
			pe.flags |= pathExpressionContainsDoubleAsterisk
			pe.legs = append(pe.legs, pathLeg{typ: pathLegDoubleAsterisk})
		}
	}
	if len(pe.legs) > 0 {
		// The last leg of a path expression cannot be '**'.
		if pe.legs[len(pe.legs)-1].typ == pathLegDoubleAsterisk {
			err = ErrInvalidJSONPath.GenWithStackByArgs(pathExpr)
			return
		}
	}
	return
}

func isBlank(c rune) bool {
	if c == '\n' || c == '\r' || c == '\t' || c == ' ' {
		return true
	}
	return false
}
