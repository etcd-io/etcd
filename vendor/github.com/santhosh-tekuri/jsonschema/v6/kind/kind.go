package kind

import (
	"fmt"
	"math/big"
	"strings"

	"golang.org/x/text/message"
)

// --

type InvalidJsonValue struct {
	Value any
}

func (*InvalidJsonValue) KeywordPath() []string {
	return nil
}

func (k *InvalidJsonValue) LocalizedString(p *message.Printer) string {
	return p.Sprintf("invalid jsonType %T", k.Value)
}

// --

type Schema struct {
	Location string
}

func (*Schema) KeywordPath() []string {
	return nil
}

func (k *Schema) LocalizedString(p *message.Printer) string {
	return p.Sprintf("jsonschema validation failed with %s", quote(k.Location))
}

// --

type Group struct{}

func (*Group) KeywordPath() []string {
	return nil
}

func (*Group) LocalizedString(p *message.Printer) string {
	return p.Sprintf("validation failed")
}

// --

type Not struct{}

func (*Not) KeywordPath() []string {
	return nil
}

func (*Not) LocalizedString(p *message.Printer) string {
	return p.Sprintf("'not' failed")
}

// --

type AllOf struct{}

func (*AllOf) KeywordPath() []string {
	return []string{"allOf"}
}

func (*AllOf) LocalizedString(p *message.Printer) string {
	return p.Sprintf("'allOf' failed")
}

// --

type AnyOf struct{}

func (*AnyOf) KeywordPath() []string {
	return []string{"anyOf"}
}

func (*AnyOf) LocalizedString(p *message.Printer) string {
	return p.Sprintf("'anyOf' failed")
}

// --

type OneOf struct {
	// Subschemas gives indexes of Subschemas that have matched.
	// Value nil, means none of the subschemas matched.
	Subschemas []int
}

func (*OneOf) KeywordPath() []string {
	return []string{"oneOf"}
}

func (k *OneOf) LocalizedString(p *message.Printer) string {
	if len(k.Subschemas) == 0 {
		return p.Sprintf("'oneOf' failed, none matched")
	}
	return p.Sprintf("'oneOf' failed, subschemas %d, %d matched", k.Subschemas[0], k.Subschemas[1])
}

//--

type FalseSchema struct{}

func (*FalseSchema) KeywordPath() []string {
	return nil
}

func (*FalseSchema) LocalizedString(p *message.Printer) string {
	return p.Sprintf("false schema")
}

// --

type RefCycle struct {
	URL              string
	KeywordLocation1 string
	KeywordLocation2 string
}

func (*RefCycle) KeywordPath() []string {
	return nil
}

func (k *RefCycle) LocalizedString(p *message.Printer) string {
	return p.Sprintf("both %s and %s resolve to %q causing reference cycle", k.KeywordLocation1, k.KeywordLocation2, k.URL)
}

// --

type Type struct {
	Got  string
	Want []string
}

func (*Type) KeywordPath() []string {
	return []string{"type"}
}

func (k *Type) LocalizedString(p *message.Printer) string {
	want := strings.Join(k.Want, " or ")
	return p.Sprintf("got %s, want %s", k.Got, want)
}

// --

type Enum struct {
	Got  any
	Want []any
}

// KeywordPath implements jsonschema.ErrorKind.
func (*Enum) KeywordPath() []string {
	return []string{"enum"}
}

func (k *Enum) LocalizedString(p *message.Printer) string {
	allPrimitive := true
loop:
	for _, item := range k.Want {
		switch item.(type) {
		case []any, map[string]any:
			allPrimitive = false
			break loop
		}
	}
	if allPrimitive {
		if len(k.Want) == 1 {
			return p.Sprintf("value must be %s", display(k.Want[0]))
		}
		var want []string
		for _, v := range k.Want {
			want = append(want, display(v))
		}
		return p.Sprintf("value must be one of %s", strings.Join(want, ", "))
	}
	return p.Sprintf("'enum' failed")
}

// --

type Const struct {
	Got  any
	Want any
}

func (*Const) KeywordPath() []string {
	return []string{"const"}
}

func (k *Const) LocalizedString(p *message.Printer) string {
	switch want := k.Want.(type) {
	case []any, map[string]any:
		return p.Sprintf("'const' failed")
	default:
		return p.Sprintf("value must be %s", display(want))
	}
}

// --

type Format struct {
	Got  any
	Want string
	Err  error
}

func (*Format) KeywordPath() []string {
	return []string{"format"}
}

func (k *Format) LocalizedString(p *message.Printer) string {
	return p.Sprintf("%s is not valid %s: %v", display(k.Got), k.Want, localizedError(k.Err, p))
}

// --

type Reference struct {
	Keyword string
	URL     string
}

func (k *Reference) KeywordPath() []string {
	return []string{k.Keyword}
}

func (*Reference) LocalizedString(p *message.Printer) string {
	return p.Sprintf("validation failed")
}

// --

type MinProperties struct {
	Got, Want int
}

func (*MinProperties) KeywordPath() []string {
	return []string{"minProperties"}
}

func (k *MinProperties) LocalizedString(p *message.Printer) string {
	return p.Sprintf("minProperties: got %d, want %d", k.Got, k.Want)
}

// --

type MaxProperties struct {
	Got, Want int
}

func (*MaxProperties) KeywordPath() []string {
	return []string{"maxProperties"}
}

func (k *MaxProperties) LocalizedString(p *message.Printer) string {
	return p.Sprintf("maxProperties: got %d, want %d", k.Got, k.Want)
}

// --

type MinItems struct {
	Got, Want int
}

func (*MinItems) KeywordPath() []string {
	return []string{"minItems"}
}

func (k *MinItems) LocalizedString(p *message.Printer) string {
	return p.Sprintf("minItems: got %d, want %d", k.Got, k.Want)
}

// --

type MaxItems struct {
	Got, Want int
}

func (*MaxItems) KeywordPath() []string {
	return []string{"maxItems"}
}

func (k *MaxItems) LocalizedString(p *message.Printer) string {
	return p.Sprintf("maxItems: got %d, want %d", k.Got, k.Want)
}

// --

type AdditionalItems struct {
	Count int
}

func (*AdditionalItems) KeywordPath() []string {
	return []string{"additionalItems"}
}

func (k *AdditionalItems) LocalizedString(p *message.Printer) string {
	return p.Sprintf("last %d additionalItem(s) not allowed", k.Count)
}

// --

type Required struct {
	Missing []string
}

func (*Required) KeywordPath() []string {
	return []string{"required"}
}

func (k *Required) LocalizedString(p *message.Printer) string {
	if len(k.Missing) == 1 {
		return p.Sprintf("missing property %s", quote(k.Missing[0]))
	}
	return p.Sprintf("missing properties %s", joinQuoted(k.Missing, ", "))
}

// --

type Dependency struct {
	Prop    string   // dependency of prop that failed
	Missing []string // missing props
}

func (k *Dependency) KeywordPath() []string {
	return []string{"dependency", k.Prop}
}

func (k *Dependency) LocalizedString(p *message.Printer) string {
	return p.Sprintf("properties %s required, if %s exists", joinQuoted(k.Missing, ", "), quote(k.Prop))
}

// --

type DependentRequired struct {
	Prop    string   // dependency of prop that failed
	Missing []string // missing props
}

func (k *DependentRequired) KeywordPath() []string {
	return []string{"dependentRequired", k.Prop}
}

func (k *DependentRequired) LocalizedString(p *message.Printer) string {
	return p.Sprintf("properties %s required, if %s exists", joinQuoted(k.Missing, ", "), quote(k.Prop))
}

// --

type AdditionalProperties struct {
	Properties []string
}

func (*AdditionalProperties) KeywordPath() []string {
	return []string{"additionalProperties"}
}

func (k *AdditionalProperties) LocalizedString(p *message.Printer) string {
	return p.Sprintf("additional properties %s not allowed", joinQuoted(k.Properties, ", "))
}

// --

type PropertyNames struct {
	Property string
}

func (*PropertyNames) KeywordPath() []string {
	return []string{"propertyNames"}
}

func (k *PropertyNames) LocalizedString(p *message.Printer) string {
	return p.Sprintf("invalid propertyName %s", quote(k.Property))
}

// --

type UniqueItems struct {
	Duplicates [2]int
}

func (*UniqueItems) KeywordPath() []string {
	return []string{"uniqueItems"}
}

func (k *UniqueItems) LocalizedString(p *message.Printer) string {
	return p.Sprintf("items at %d and %d are equal", k.Duplicates[0], k.Duplicates[1])
}

// --

type Contains struct{}

func (*Contains) KeywordPath() []string {
	return []string{"contains"}
}

func (*Contains) LocalizedString(p *message.Printer) string {
	return p.Sprintf("no items match contains schema")
}

// --

type MinContains struct {
	Got  []int
	Want int
}

func (*MinContains) KeywordPath() []string {
	return []string{"minContains"}
}

func (k *MinContains) LocalizedString(p *message.Printer) string {
	if len(k.Got) == 0 {
		return p.Sprintf("min %d items required to match contains schema, but none matched", k.Want)
	} else {
		got := fmt.Sprintf("%v", k.Got)
		return p.Sprintf("min %d items required to match contains schema, but matched %d items at %v", k.Want, len(k.Got), got[1:len(got)-1])
	}
}

// --

type MaxContains struct {
	Got  []int
	Want int
}

func (*MaxContains) KeywordPath() []string {
	return []string{"maxContains"}
}

func (k *MaxContains) LocalizedString(p *message.Printer) string {
	got := fmt.Sprintf("%v", k.Got)
	return p.Sprintf("max %d items required to match contains schema, but matched %d items at %v", k.Want, len(k.Got), got[1:len(got)-1])
}

// --

type MinLength struct {
	Got, Want int
}

func (*MinLength) KeywordPath() []string {
	return []string{"minLength"}
}

func (k *MinLength) LocalizedString(p *message.Printer) string {
	return p.Sprintf("minLength: got %d, want %d", k.Got, k.Want)
}

// --

type MaxLength struct {
	Got, Want int
}

func (*MaxLength) KeywordPath() []string {
	return []string{"maxLength"}
}

func (k *MaxLength) LocalizedString(p *message.Printer) string {
	return p.Sprintf("maxLength: got %d, want %d", k.Got, k.Want)
}

// --

type Pattern struct {
	Got  string
	Want string
}

func (*Pattern) KeywordPath() []string {
	return []string{"pattern"}
}

func (k *Pattern) LocalizedString(p *message.Printer) string {
	return p.Sprintf("%s does not match pattern %s", quote(k.Got), quote(k.Want))
}

// --

type ContentEncoding struct {
	Want string
	Err  error
}

func (*ContentEncoding) KeywordPath() []string {
	return []string{"contentEncoding"}
}

func (k *ContentEncoding) LocalizedString(p *message.Printer) string {
	return p.Sprintf("value is not %s encoded: %v", quote(k.Want), localizedError(k.Err, p))
}

// --

type ContentMediaType struct {
	Got  []byte
	Want string
	Err  error
}

func (*ContentMediaType) KeywordPath() []string {
	return []string{"contentMediaType"}
}

func (k *ContentMediaType) LocalizedString(p *message.Printer) string {
	return p.Sprintf("value if not of mediatype %s: %v", quote(k.Want), k.Err)
}

// --

type ContentSchema struct{}

func (*ContentSchema) KeywordPath() []string {
	return []string{"contentSchema"}
}

func (*ContentSchema) LocalizedString(p *message.Printer) string {
	return p.Sprintf("'contentSchema' failed")
}

// --

type Minimum struct {
	Got  *big.Rat
	Want *big.Rat
}

func (*Minimum) KeywordPath() []string {
	return []string{"minimum"}
}

func (k *Minimum) LocalizedString(p *message.Printer) string {
	got, _ := k.Got.Float64()
	want, _ := k.Want.Float64()
	return p.Sprintf("minimum: got %v, want %v", got, want)
}

// --

type Maximum struct {
	Got  *big.Rat
	Want *big.Rat
}

func (*Maximum) KeywordPath() []string {
	return []string{"maximum"}
}

func (k *Maximum) LocalizedString(p *message.Printer) string {
	got, _ := k.Got.Float64()
	want, _ := k.Want.Float64()
	return p.Sprintf("maximum: got %v, want %v", got, want)
}

// --

type ExclusiveMinimum struct {
	Got  *big.Rat
	Want *big.Rat
}

func (*ExclusiveMinimum) KeywordPath() []string {
	return []string{"exclusiveMinimum"}
}

func (k *ExclusiveMinimum) LocalizedString(p *message.Printer) string {
	got, _ := k.Got.Float64()
	want, _ := k.Want.Float64()
	return p.Sprintf("exclusiveMinimum: got %v, want %v", got, want)
}

// --

type ExclusiveMaximum struct {
	Got  *big.Rat
	Want *big.Rat
}

func (*ExclusiveMaximum) KeywordPath() []string {
	return []string{"exclusiveMaximum"}
}

func (k *ExclusiveMaximum) LocalizedString(p *message.Printer) string {
	got, _ := k.Got.Float64()
	want, _ := k.Want.Float64()
	return p.Sprintf("exclusiveMaximum: got %v, want %v", got, want)
}

// --

type MultipleOf struct {
	Got  *big.Rat
	Want *big.Rat
}

func (*MultipleOf) KeywordPath() []string {
	return []string{"multipleOf"}
}

func (k *MultipleOf) LocalizedString(p *message.Printer) string {
	got, _ := k.Got.Float64()
	want, _ := k.Want.Float64()
	return p.Sprintf("multipleOf: got %v, want %v", got, want)
}

// --

func quote(s string) string {
	s = fmt.Sprintf("%q", s)
	s = strings.ReplaceAll(s, `\"`, `"`)
	s = strings.ReplaceAll(s, `'`, `\'`)
	return "'" + s[1:len(s)-1] + "'"
}

func joinQuoted(arr []string, sep string) string {
	var sb strings.Builder
	for _, s := range arr {
		if sb.Len() > 0 {
			sb.WriteString(sep)
		}
		sb.WriteString(quote(s))
	}
	return sb.String()
}

// to be used only for primitive.
func display(v any) string {
	switch v := v.(type) {
	case string:
		return quote(v)
	case []any, map[string]any:
		return "value"
	default:
		return fmt.Sprintf("%v", v)
	}
}

func localizedError(err error, p *message.Printer) string {
	if err, ok := err.(interface{ LocalizedError(*message.Printer) string }); ok {
		return err.LocalizedError(p)
	}
	return err.Error()
}
