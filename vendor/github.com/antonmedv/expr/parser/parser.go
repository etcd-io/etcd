package parser

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	. "github.com/antonmedv/expr/ast"
	"github.com/antonmedv/expr/file"
	. "github.com/antonmedv/expr/parser/lexer"
)

type associativity int

const (
	left associativity = iota + 1
	right
)

type operator struct {
	precedence    int
	associativity associativity
}

type builtin struct {
	arity int
}

var unaryOperators = map[string]operator{
	"not": {50, left},
	"!":   {50, left},
	"-":   {500, left},
	"+":   {500, left},
}

var binaryOperators = map[string]operator{
	"or":         {10, left},
	"||":         {10, left},
	"and":        {15, left},
	"&&":         {15, left},
	"==":         {20, left},
	"!=":         {20, left},
	"<":          {20, left},
	">":          {20, left},
	">=":         {20, left},
	"<=":         {20, left},
	"not in":     {20, left},
	"in":         {20, left},
	"matches":    {20, left},
	"contains":   {20, left},
	"startsWith": {20, left},
	"endsWith":   {20, left},
	"..":         {25, left},
	"+":          {30, left},
	"-":          {30, left},
	"*":          {60, left},
	"/":          {60, left},
	"%":          {60, left},
	"**":         {70, right},
}

var builtins = map[string]builtin{
	"len":    {1},
	"all":    {2},
	"none":   {2},
	"any":    {2},
	"one":    {2},
	"filter": {2},
	"map":    {2},
	"count":  {2},
}

type parser struct {
	tokens  []Token
	current Token
	pos     int
	err     *file.Error
	depth   int // closure call depth
}

type Tree struct {
	Node   Node
	Source *file.Source
}

func Parse(input string) (*Tree, error) {
	source := file.NewSource(input)

	tokens, err := Lex(source)
	if err != nil {
		return nil, err
	}

	p := &parser{
		tokens:  tokens,
		current: tokens[0],
	}

	node := p.parseExpression(0)

	if !p.current.Is(EOF) {
		p.error("unexpected token %v", p.current)
	}

	if p.err != nil {
		return nil, p.err.Bind(source)
	}

	return &Tree{
		Node:   node,
		Source: source,
	}, nil
}

func (p *parser) error(format string, args ...interface{}) {
	if p.err == nil { // show first error
		p.err = &file.Error{
			Location: p.current.Location,
			Message:  fmt.Sprintf(format, args...),
		}
	}
}

func (p *parser) next() {
	p.pos++
	if p.pos >= len(p.tokens) {
		p.error("unexpected end of expression")
		return
	}
	p.current = p.tokens[p.pos]
}

func (p *parser) expect(kind Kind, values ...string) {
	if p.current.Is(kind, values...) {
		p.next()
		return
	}
	p.error("unexpected token %v", p.current)
}

// parse functions

func (p *parser) parseExpression(precedence int) Node {
	nodeLeft := p.parsePrimary()

	token := p.current
	for token.Is(Operator) && p.err == nil {
		if op, ok := binaryOperators[token.Value]; ok {
			if op.precedence >= precedence {
				p.next()

				var nodeRight Node
				if op.associativity == left {
					nodeRight = p.parseExpression(op.precedence + 1)
				} else {
					nodeRight = p.parseExpression(op.precedence)
				}

				if token.Is(Operator, "matches") {
					var r *regexp.Regexp
					var err error

					if s, ok := nodeRight.(*StringNode); ok {
						r, err = regexp.Compile(s.Value)
						if err != nil {
							p.error("%v", err)
						}
					}
					nodeLeft = &MatchesNode{
						Regexp: r,
						Left:   nodeLeft,
						Right:  nodeRight,
					}
					nodeLeft.SetLocation(token.Location)
				} else {
					nodeLeft = &BinaryNode{
						Operator: token.Value,
						Left:     nodeLeft,
						Right:    nodeRight,
					}
					nodeLeft.SetLocation(token.Location)
				}
				token = p.current
				continue
			}
		}
		break
	}

	if precedence == 0 {
		nodeLeft = p.parseConditionalExpression(nodeLeft)
	}

	return nodeLeft
}

func (p *parser) parsePrimary() Node {
	token := p.current

	if token.Is(Operator) {
		if op, ok := unaryOperators[token.Value]; ok {
			p.next()
			expr := p.parseExpression(op.precedence)
			node := &UnaryNode{
				Operator: token.Value,
				Node:     expr,
			}
			node.SetLocation(token.Location)
			return p.parsePostfixExpression(node)
		}
	}

	if token.Is(Bracket, "(") {
		p.next()
		expr := p.parseExpression(0)
		p.expect(Bracket, ")") // "an opened parenthesis is not properly closed"
		return p.parsePostfixExpression(expr)
	}

	if p.depth > 0 {
		if token.Is(Operator, "#") || token.Is(Operator, ".") {
			if token.Is(Operator, "#") {
				p.next()
			}
			node := &PointerNode{}
			node.SetLocation(token.Location)
			return p.parsePostfixExpression(node)
		}
	} else {
		if token.Is(Operator, "#") || token.Is(Operator, ".") {
			p.error("cannot use pointer accessor outside closure")
		}
	}

	return p.parsePrimaryExpression()
}

func (p *parser) parseConditionalExpression(node Node) Node {
	var expr1, expr2 Node
	for p.current.Is(Operator, "?") && p.err == nil {
		p.next()

		if !p.current.Is(Operator, ":") {
			expr1 = p.parseExpression(0)
			p.expect(Operator, ":")
			expr2 = p.parseExpression(0)
		} else {
			p.next()
			expr1 = node
			expr2 = p.parseExpression(0)
		}

		node = &ConditionalNode{
			Cond: node,
			Exp1: expr1,
			Exp2: expr2,
		}
	}
	return node
}

func (p *parser) parsePrimaryExpression() Node {
	var node Node
	token := p.current

	switch token.Kind {

	case Identifier:
		p.next()
		switch token.Value {
		case "true":
			node := &BoolNode{Value: true}
			node.SetLocation(token.Location)
			return node
		case "false":
			node := &BoolNode{Value: false}
			node.SetLocation(token.Location)
			return node
		case "nil":
			node := &NilNode{}
			node.SetLocation(token.Location)
			return node
		default:
			node = p.parseIdentifierExpression(token)
		}

	case Number:
		p.next()
		value := strings.Replace(token.Value, "_", "", -1)
		if strings.ContainsAny(value, ".eE") {
			number, err := strconv.ParseFloat(value, 64)
			if err != nil {
				p.error("invalid float literal: %v", err)
			}
			node := &FloatNode{Value: number}
			node.SetLocation(token.Location)
			return node
		} else if strings.Contains(value, "x") {
			number, err := strconv.ParseInt(value, 0, 64)
			if err != nil {
				p.error("invalid hex literal: %v", err)
			}
			node := &IntegerNode{Value: int(number)}
			node.SetLocation(token.Location)
			return node
		} else {
			number, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				p.error("invalid integer literal: %v", err)
			}
			node := &IntegerNode{Value: int(number)}
			node.SetLocation(token.Location)
			return node
		}

	case String:
		p.next()
		node := &StringNode{Value: token.Value}
		node.SetLocation(token.Location)
		return node

	default:
		if token.Is(Bracket, "[") {
			node = p.parseArrayExpression(token)
		} else if token.Is(Bracket, "{") {
			node = p.parseMapExpression(token)
		} else {
			p.error("unexpected token %v", token)
		}
	}

	return p.parsePostfixExpression(node)
}

func (p *parser) parseIdentifierExpression(token Token) Node {
	var node Node
	if p.current.Is(Bracket, "(") {
		var arguments []Node

		if b, ok := builtins[token.Value]; ok {
			p.expect(Bracket, "(")
			// TODO: Add builtins signatures.
			if b.arity == 1 {
				arguments = make([]Node, 1)
				arguments[0] = p.parseExpression(0)
			} else if b.arity == 2 {
				arguments = make([]Node, 2)
				arguments[0] = p.parseExpression(0)
				p.expect(Operator, ",")
				arguments[1] = p.parseClosure()
			}
			p.expect(Bracket, ")")

			node = &BuiltinNode{
				Name:      token.Value,
				Arguments: arguments,
			}
			node.SetLocation(token.Location)
		} else {
			arguments = p.parseArguments()
			node = &FunctionNode{
				Name:      token.Value,
				Arguments: arguments,
			}
			node.SetLocation(token.Location)
		}
	} else {
		node = &IdentifierNode{Value: token.Value}
		node.SetLocation(token.Location)
	}
	return node
}

func (p *parser) parseClosure() Node {
	token := p.current
	p.expect(Bracket, "{")

	p.depth++
	node := p.parseExpression(0)
	p.depth--

	p.expect(Bracket, "}")
	closure := &ClosureNode{
		Node: node,
	}
	closure.SetLocation(token.Location)
	return closure
}

func (p *parser) parseArrayExpression(token Token) Node {
	nodes := make([]Node, 0)

	p.expect(Bracket, "[")
	for !p.current.Is(Bracket, "]") && p.err == nil {
		if len(nodes) > 0 {
			p.expect(Operator, ",")
			if p.current.Is(Bracket, "]") {
				goto end
			}
		}
		node := p.parseExpression(0)
		nodes = append(nodes, node)
	}
end:
	p.expect(Bracket, "]")

	node := &ArrayNode{Nodes: nodes}
	node.SetLocation(token.Location)
	return node
}

func (p *parser) parseMapExpression(token Token) Node {
	p.expect(Bracket, "{")

	nodes := make([]Node, 0)
	for !p.current.Is(Bracket, "}") && p.err == nil {
		if len(nodes) > 0 {
			p.expect(Operator, ",")
			if p.current.Is(Bracket, "}") {
				goto end
			}
			if p.current.Is(Operator, ",") {
				p.error("unexpected token %v", p.current)
			}
		}

		var key Node
		// a map key can be:
		//  * a number
		//  * a string
		//  * a identifier, which is equivalent to a string
		//  * an expression, which must be enclosed in parentheses -- (1 + 2)
		if p.current.Is(Number) || p.current.Is(String) || p.current.Is(Identifier) {
			key = &StringNode{Value: p.current.Value}
			key.SetLocation(token.Location)
			p.next()
		} else if p.current.Is(Bracket, "(") {
			key = p.parseExpression(0)
		} else {
			p.error("a map key must be a quoted string, a number, a identifier, or an expression enclosed in parentheses (unexpected token %v)", p.current)
		}

		p.expect(Operator, ":")

		node := p.parseExpression(0)
		pair := &PairNode{Key: key, Value: node}
		pair.SetLocation(token.Location)
		nodes = append(nodes, pair)
	}

end:
	p.expect(Bracket, "}")

	node := &MapNode{Pairs: nodes}
	node.SetLocation(token.Location)
	return node
}

func (p *parser) parsePostfixExpression(node Node) Node {
	token := p.current
	for (token.Is(Operator) || token.Is(Bracket)) && p.err == nil {
		if token.Value == "." {
			p.next()

			token = p.current
			p.next()

			if token.Kind != Identifier &&
				// Operators like "not" and "matches" are valid methods or property names.
				(token.Kind != Operator || !isValidIdentifier(token.Value)) {
				p.error("expected name")
			}

			if p.current.Is(Bracket, "(") {
				arguments := p.parseArguments()
				node = &MethodNode{
					Node:      node,
					Method:    token.Value,
					Arguments: arguments,
				}
				node.SetLocation(token.Location)
			} else {
				node = &PropertyNode{
					Node:     node,
					Property: token.Value,
				}
				node.SetLocation(token.Location)
			}

		} else if token.Value == "[" {
			p.next()
			var from, to Node

			if p.current.Is(Operator, ":") { // slice without from [:1]
				p.next()

				if !p.current.Is(Bracket, "]") { // slice without from and to [:]
					to = p.parseExpression(0)
				}

				node = &SliceNode{
					Node: node,
					To:   to,
				}
				node.SetLocation(token.Location)
				p.expect(Bracket, "]")

			} else {

				from = p.parseExpression(0)

				if p.current.Is(Operator, ":") {
					p.next()

					if !p.current.Is(Bracket, "]") { // slice without to [1:]
						to = p.parseExpression(0)
					}

					node = &SliceNode{
						Node: node,
						From: from,
						To:   to,
					}
					node.SetLocation(token.Location)
					p.expect(Bracket, "]")

				} else {
					// Slice operator [:] was not found, it should by just index node.

					node = &IndexNode{
						Node:  node,
						Index: from,
					}
					node.SetLocation(token.Location)
					p.expect(Bracket, "]")
				}
			}

		} else {
			break
		}

		token = p.current
	}
	return node
}

func isValidIdentifier(str string) bool {
	if len(str) == 0 {
		return false
	}
	h, w := utf8.DecodeRuneInString(str)
	if !IsAlphabetic(h) {
		return false
	}
	for _, r := range str[w:] {
		if !IsAlphaNumeric(r) {
			return false
		}
	}
	return true
}

func (p *parser) parseArguments() []Node {
	p.expect(Bracket, "(")
	nodes := make([]Node, 0)
	for !p.current.Is(Bracket, ")") && p.err == nil {
		if len(nodes) > 0 {
			p.expect(Operator, ",")
		}
		node := p.parseExpression(0)
		nodes = append(nodes, node)
	}
	p.expect(Bracket, ")")

	return nodes
}
