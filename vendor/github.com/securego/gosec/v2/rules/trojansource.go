package rules

import (
	"go/ast"
	"os"

	"github.com/securego/gosec/v2"
	"github.com/securego/gosec/v2/issue"
)

type trojanSource struct {
	issue.MetaData
	bidiChars map[rune]struct{}
}

func (r *trojanSource) Match(node ast.Node, c *gosec.Context) (*issue.Issue, error) {
	if file, ok := node.(*ast.File); ok {
		fobj := c.FileSet.File(file.Pos())
		if fobj == nil {
			return nil, nil
		}

		content, err := os.ReadFile(fobj.Name())
		if err != nil {
			return nil, nil
		}

		for _, ch := range string(content) {
			if _, exists := r.bidiChars[ch]; exists {
				return c.NewIssue(node, r.ID(), r.What, r.Severity, r.Confidence), nil
			}
		}
	}

	return nil, nil
}

// func (r *trojanSource) Match(node ast.Node, c *gosec.Context) (*issue.Issue, error) {
// 	if file, ok := node.(*ast.File); ok {
// 		fobj := c.FileSet.File(file.Pos())
// 		if fobj == nil {
// 			return nil, nil
// 		}

// 		file, err := os.Open(fobj.Name())
// 		if err != nil {
// 			log.Fatal(err)
// 		}

// 		defer file.Close()

// 		scanner := bufio.NewScanner(file)
// 		for scanner.Scan() {
// 			line := scanner.Text()
// 			for _, ch := range line {
// 				if _, exists := r.bidiChars[ch]; exists {
// 					return c.NewIssue(node, r.ID(), r.What, r.Severity, r.Confidence), nil
// 				}
// 			}
// 		}

// 		if err := scanner.Err(); err != nil {
// 			log.Fatal(err)
// 		}
// 	}

// 	return nil, nil
// }

func NewTrojanSource(id string, _ gosec.Config) (gosec.Rule, []ast.Node) {
	return &trojanSource{
		MetaData: issue.NewMetaData(id, "Potential Trojan Source vulnerability via use of bidirectional text control characters", issue.High, issue.Medium),
		bidiChars: map[rune]struct{}{
			'\u202a': {},
			'\u202b': {},
			'\u202c': {},
			'\u202d': {},
			'\u202e': {},
			'\u2066': {},
			'\u2067': {},
			'\u2068': {},
			'\u2069': {},
			'\u200e': {},
			'\u200f': {},
		},
	}, []ast.Node{(*ast.File)(nil)}
}
