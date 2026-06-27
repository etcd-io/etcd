package jsonschema

// CompilerContext provides helpers for
// compiling a [Vocabulary].
type CompilerContext struct {
	c *objCompiler
}

func (ctx *CompilerContext) Enqueue(schPath []string) *Schema {
	ptr := ctx.c.up.ptr
	for _, tok := range schPath {
		ptr = ptr.append(tok)
	}
	return ctx.c.enqueuePtr(ptr)
}

// Vocabulary defines a set of keywords, their syntax and
// their semantics.
type Vocabulary struct {
	// URL identifier for this Vocabulary.
	URL string

	// Schema that is used to validate the keywords that is introduced by this
	// vocabulary.
	Schema *Schema

	// Subschemas lists the possible locations of subschemas introduced by
	// this vocabulary.
	Subschemas []SchemaPath

	// Compile compiles the keywords(introduced by this vocabulary) in obj into [SchemaExt].
	// If obj does not contain any keywords introduced by this vocabulary, nil SchemaExt must
	// be returned.
	Compile func(ctx *CompilerContext, obj map[string]any) (SchemaExt, error)
}

// --

// SchemaExt is compled form of vocabulary.
type SchemaExt interface {
	// Validate validates v against and errors if any are reported
	// to ctx.
	Validate(ctx *ValidatorContext, v any)
}

// ValidatorContext provides helpers for
// validating with [SchemaExt].
type ValidatorContext struct {
	vd *validator
}

// ValueLocation returns location of value as jsonpath token array.
func (ctx *ValidatorContext) ValueLocation() []string {
	return ctx.vd.vloc
}

// Validate validates v with sch. vpath gives path of v from current context value.
func (ctx *ValidatorContext) Validate(sch *Schema, v any, vpath []string) error {
	switch len(vpath) {
	case 0:
		return ctx.vd.validateSelf(sch, "", false)
	case 1:
		return ctx.vd.validateVal(sch, v, vpath[0])
	default:
		return ctx.vd.validateValue(sch, v, vpath)
	}
}

// EvaluatedProp marks given property of current object as evaluated.
func (ctx *ValidatorContext) EvaluatedProp(pname string) {
	delete(ctx.vd.uneval.props, pname)
}

// EvaluatedItem marks items at given index of current array as evaluated.
func (ctx *ValidatorContext) EvaluatedItem(index int) {
	delete(ctx.vd.uneval.items, index)
}

// AddError reports validation-error of given kind.
func (ctx *ValidatorContext) AddError(k ErrorKind) {
	ctx.vd.addError(k)
}

// AddErrors reports validation-errors of given kind.
func (ctx *ValidatorContext) AddErrors(errors []*ValidationError, k ErrorKind) {
	ctx.vd.addErrors(errors, k)
}

// AddErr reports the given err. This is typically used to report
// the error created by subschema validation.
//
// NOTE that err must be of type *ValidationError.
func (ctx *ValidatorContext) AddErr(err error) {
	ctx.vd.addErr(err)
}

func (ctx *ValidatorContext) Equals(v1, v2 any) (bool, error) {
	b, k := equals(v1, v2)
	if k != nil {
		return false, ctx.vd.error(k)
	}
	return b, nil
}

func (ctx *ValidatorContext) Duplicates(arr []any) (int, int, error) {
	i, j, k := duplicates(arr)
	if k != nil {
		return -1, -1, ctx.vd.error(k)
	}
	return i, j, nil
}
