package structure

import (
	"go/ast"
	"go/types"
	"reflect"
	"strings"
)

const (
	tagName          = "exhaustruct"
	optionalTagValue = "optional"
)

type Field struct {
	Name     string
	Exported bool
	Optional bool
}

type Fields []*Field

// NewFields creates a new [Fields] from a given struct type.
// Fields items are listed in order they appear in the struct.
func NewFields(strct *types.Struct) Fields {
	sf := make(Fields, 0, strct.NumFields())

	for i := 0; i < strct.NumFields(); i++ {
		f := strct.Field(i)

		sf = append(sf, &Field{
			Name:     f.Name(),
			Exported: f.Exported(),
			Optional: HasOptionalTag(strct.Tag(i)),
		})
	}

	return sf
}

func HasOptionalTag(tags string) bool {
	return reflect.StructTag(tags).Get(tagName) == optionalTagValue
}

// String returns a comma-separated list of field names.
func (sf Fields) String() string {
	b := strings.Builder{}

	for i := 0; i < len(sf); i++ {
		if b.Len() != 0 {
			b.WriteString(", ")
		}

		b.WriteString(sf[i].Name)
	}

	return b.String()
}

// Skipped returns a list of fields that are not present in the given
// literal, but expected to.
//
//revive:disable-next-line:cyclomatic
func (sf Fields) Skipped(lit *ast.CompositeLit, onlyExported bool) Fields {
	if len(lit.Elts) != 0 && !isNamedLiteral(lit) {
		if len(lit.Elts) == len(sf) {
			return nil
		}

		return sf[len(lit.Elts):]
	}

	em := sf.existenceMap()
	res := make(Fields, 0, len(sf))

	for i := 0; i < len(lit.Elts); i++ {
		kv, ok := lit.Elts[i].(*ast.KeyValueExpr)
		if !ok {
			continue
		}

		k, ok := kv.Key.(*ast.Ident)
		if !ok {
			continue
		}

		em[k.Name] = true
	}

	for i := 0; i < len(sf); i++ {
		if em[sf[i].Name] || (!sf[i].Exported && onlyExported) || sf[i].Optional {
			continue
		}

		res = append(res, sf[i])
	}

	if len(res) == 0 {
		return nil
	}

	return res
}

func (sf Fields) existenceMap() map[string]bool {
	m := make(map[string]bool, len(sf))

	for i := 0; i < len(sf); i++ {
		m[sf[i].Name] = false
	}

	return m
}

// isNamedLiteral returns true if the given literal is unnamed.
//
// The logic is basing on the principle that literal is named or unnamed,
// therefore is literal's first element is a [ast.KeyValueExpr], it is named.
//
// Method will panic if the given literal is empty.
func isNamedLiteral(lit *ast.CompositeLit) bool {
	if _, ok := lit.Elts[0].(*ast.KeyValueExpr); !ok {
		return false
	}

	return true
}
