package jsonschema

import (
	"encoding/json"
	"fmt"
	"math/big"
	"slices"
	"strconv"
	"unicode/utf8"

	"github.com/santhosh-tekuri/jsonschema/v6/kind"
	"golang.org/x/text/message"
)

func (sch *Schema) Validate(v any) error {
	return sch.validate(v, nil, nil, nil, false, nil)
}

func (sch *Schema) validate(v any, regexpEngine RegexpEngine, meta *Schema, resources map[jsonPointer]*resource, assertVocabs bool, vocabularies map[string]*Vocabulary) error {
	vd := validator{
		v:            v,
		vloc:         make([]string, 0, 8),
		sch:          sch,
		scp:          &scope{sch, "", 0, nil},
		uneval:       unevalFrom(v, sch, false),
		errors:       nil,
		boolResult:   false,
		regexpEngine: regexpEngine,
		meta:         meta,
		resources:    resources,
		assertVocabs: assertVocabs,
		vocabularies: vocabularies,
	}
	if _, err := vd.validate(); err != nil {
		verr := err.(*ValidationError)
		var causes []*ValidationError
		if _, ok := verr.ErrorKind.(*kind.Group); ok {
			causes = verr.Causes
		} else {
			causes = []*ValidationError{verr}
		}
		return &ValidationError{
			SchemaURL:        sch.Location,
			InstanceLocation: nil,
			ErrorKind:        &kind.Schema{Location: sch.Location},
			Causes:           causes,
		}
	}

	return nil
}

type validator struct {
	v            any
	vloc         []string
	sch          *Schema
	scp          *scope
	uneval       *uneval
	errors       []*ValidationError
	boolResult   bool // is interested to know valid or not (but not actuall error)
	regexpEngine RegexpEngine

	// meta validation
	meta         *Schema                   // set only when validating with metaschema
	resources    map[jsonPointer]*resource // resources which should be validated with their dialect
	assertVocabs bool
	vocabularies map[string]*Vocabulary
}

func (vd *validator) validate() (*uneval, error) {
	s := vd.sch
	v := vd.v

	// boolean --
	if s.Bool != nil {
		if *s.Bool {
			return vd.uneval, nil
		} else {
			return nil, vd.error(&kind.FalseSchema{})
		}
	}

	// check cycle --
	if scp := vd.scp.checkCycle(); scp != nil {
		return nil, vd.error(&kind.RefCycle{
			URL:              s.Location,
			KeywordLocation1: vd.scp.kwLoc(),
			KeywordLocation2: scp.kwLoc(),
		})
	}

	t := typeOf(v)
	if t == invalidType {
		return nil, vd.error(&kind.InvalidJsonValue{Value: v})
	}

	// type --
	if s.Types != nil && !s.Types.IsEmpty() {
		matched := s.Types.contains(t) || (s.Types.contains(integerType) && t == numberType && isInteger(v))
		if !matched {
			return nil, vd.error(&kind.Type{Got: t.String(), Want: s.Types.ToStrings()})
		}
	}

	// const --
	if s.Const != nil {
		ok, k := equals(v, *s.Const)
		if k != nil {
			return nil, vd.error(k)
		} else if !ok {
			return nil, vd.error(&kind.Const{Got: v, Want: *s.Const})
		}
	}

	// enum --
	if s.Enum != nil {
		matched := s.Enum.types.contains(typeOf(v))
		if matched {
			matched = false
			for _, item := range s.Enum.Values {
				ok, k := equals(v, item)
				if k != nil {
					return nil, vd.error(k)
				} else if ok {
					matched = true
					break
				}
			}
		}
		if !matched {
			return nil, vd.error(&kind.Enum{Got: v, Want: s.Enum.Values})
		}
	}

	// format --
	if s.Format != nil {
		var err error
		if s.Format.Name == "regex" && vd.regexpEngine != nil {
			err = vd.regexpEngine.validate(v)
		} else {
			err = s.Format.Validate(v)
		}
		if err != nil {
			return nil, vd.error(&kind.Format{Got: v, Want: s.Format.Name, Err: err})
		}
	}

	// $ref --
	if s.Ref != nil {
		err := vd.validateRef(s.Ref, "$ref")
		if s.DraftVersion < 2019 {
			return vd.uneval, err
		}
		if err != nil {
			vd.addErr(err)
		}
	}

	// type specific validations --
	switch v := v.(type) {
	case map[string]any:
		vd.objValidate(v)
	case []any:
		vd.arrValidate(v)
	case string:
		vd.strValidate(v)
	case json.Number, float32, float64, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		vd.numValidate(v)
	}

	if len(vd.errors) == 0 || !vd.boolResult {
		if s.DraftVersion >= 2019 {
			vd.validateRefs()
		}
		vd.condValidate()

		for _, ext := range s.Extensions {
			ext.Validate(&ValidatorContext{vd}, v)
		}

		if s.DraftVersion >= 2019 {
			vd.unevalValidate()
		}
	}

	switch len(vd.errors) {
	case 0:
		return vd.uneval, nil
	case 1:
		return nil, vd.errors[0]
	default:
		verr := vd.error(&kind.Group{})
		verr.Causes = vd.errors
		return nil, verr
	}
}

func (vd *validator) objValidate(obj map[string]any) {
	s := vd.sch

	// minProperties --
	if s.MinProperties != nil {
		if len(obj) < *s.MinProperties {
			vd.addError(&kind.MinProperties{Got: len(obj), Want: *s.MinProperties})
		}
	}

	// maxProperties --
	if s.MaxProperties != nil {
		if len(obj) > *s.MaxProperties {
			vd.addError(&kind.MaxProperties{Got: len(obj), Want: *s.MaxProperties})
		}
	}

	// required --
	if len(s.Required) > 0 {
		if missing := vd.findMissing(obj, s.Required); missing != nil {
			vd.addError(&kind.Required{Missing: missing})
		}
	}

	if vd.boolResult && len(vd.errors) > 0 {
		return
	}

	// dependencies --
	for pname, dep := range s.Dependencies {
		if _, ok := obj[pname]; ok {
			switch dep := dep.(type) {
			case []string:
				if missing := vd.findMissing(obj, dep); missing != nil {
					vd.addError(&kind.Dependency{Prop: pname, Missing: missing})
				}
			case *Schema:
				vd.addErr(vd.validateSelf(dep, "", false))
			}
		}
	}

	var additionalPros []string
	for pname, pvalue := range obj {
		if vd.boolResult && len(vd.errors) > 0 {
			return
		}
		evaluated := false

		// properties --
		if sch, ok := s.Properties[pname]; ok {
			evaluated = true
			vd.addErr(vd.validateVal(sch, pvalue, pname))
		}

		// patternProperties --
		for regex, sch := range s.PatternProperties {
			if regex.MatchString(pname) {
				evaluated = true
				vd.addErr(vd.validateVal(sch, pvalue, pname))
			}
		}

		if !evaluated && s.AdditionalProperties != nil {
			evaluated = true
			switch additional := s.AdditionalProperties.(type) {
			case bool:
				if !additional {
					additionalPros = append(additionalPros, pname)
				}
			case *Schema:
				vd.addErr(vd.validateVal(additional, pvalue, pname))
			}
		}

		if evaluated {
			delete(vd.uneval.props, pname)
		}
	}
	if len(additionalPros) > 0 {
		vd.addError(&kind.AdditionalProperties{Properties: additionalPros})
	}

	if s.DraftVersion == 4 {
		return
	}

	// propertyNames --
	if s.PropertyNames != nil {
		for pname := range obj {
			sch, meta, resources := s.PropertyNames, vd.meta, vd.resources
			res := vd.metaResource(sch)
			if res != nil {
				meta = res.dialect.getSchema(vd.assertVocabs, vd.vocabularies)
				sch = meta
			}
			if err := sch.validate(pname, vd.regexpEngine, meta, resources, vd.assertVocabs, vd.vocabularies); err != nil {
				verr := err.(*ValidationError)
				verr.SchemaURL = s.PropertyNames.Location
				verr.ErrorKind = &kind.PropertyNames{Property: pname}
				vd.addErr(verr)
			}
		}
	}

	if s.DraftVersion == 6 {
		return
	}

	// dependentSchemas --
	for pname, sch := range s.DependentSchemas {
		if _, ok := obj[pname]; ok {
			vd.addErr(vd.validateSelf(sch, "", false))
		}
	}

	// dependentRequired --
	for pname, reqd := range s.DependentRequired {
		if _, ok := obj[pname]; ok {
			if missing := vd.findMissing(obj, reqd); missing != nil {
				vd.addError(&kind.DependentRequired{Prop: pname, Missing: missing})
			}
		}
	}
}

func (vd *validator) arrValidate(arr []any) {
	s := vd.sch

	// minItems --
	if s.MinItems != nil {
		if len(arr) < *s.MinItems {
			vd.addError(&kind.MinItems{Got: len(arr), Want: *s.MinItems})
		}
	}

	// maxItems --
	if s.MaxItems != nil {
		if len(arr) > *s.MaxItems {
			vd.addError(&kind.MaxItems{Got: len(arr), Want: *s.MaxItems})
		}
	}

	// uniqueItems --
	if s.UniqueItems && len(arr) > 1 {
		i, j, k := duplicates(arr)
		if k != nil {
			vd.addError(k)
		} else if i != -1 {
			vd.addError(&kind.UniqueItems{Duplicates: [2]int{i, j}})
		}
	}

	if s.DraftVersion < 2020 {
		evaluated := 0

		// items --
		switch items := s.Items.(type) {
		case *Schema:
			for i, item := range arr {
				vd.addErr(vd.validateVal(items, item, strconv.Itoa(i)))
			}
			evaluated = len(arr)
		case []*Schema:
			min := minInt(len(arr), len(items))
			for i, item := range arr[:min] {
				vd.addErr(vd.validateVal(items[i], item, strconv.Itoa(i)))
			}
			evaluated = min
		}

		// additionalItems --
		if s.AdditionalItems != nil {
			switch additional := s.AdditionalItems.(type) {
			case bool:
				if !additional && evaluated != len(arr) {
					vd.addError(&kind.AdditionalItems{Count: len(arr) - evaluated})
				}
			case *Schema:
				for i, item := range arr[evaluated:] {
					vd.addErr(vd.validateVal(additional, item, strconv.Itoa(i)))
				}
			}
		}
	} else {
		evaluated := minInt(len(s.PrefixItems), len(arr))

		// prefixItems --
		for i, item := range arr[:evaluated] {
			vd.addErr(vd.validateVal(s.PrefixItems[i], item, strconv.Itoa(i)))
		}

		// items2020 --
		if s.Items2020 != nil {
			for i, item := range arr[evaluated:] {
				vd.addErr(vd.validateVal(s.Items2020, item, strconv.Itoa(i)))
			}
		}
	}

	// contains --
	if s.Contains != nil {
		var errors []*ValidationError
		var matched []int

		for i, item := range arr {
			if err := vd.validateVal(s.Contains, item, strconv.Itoa(i)); err != nil {
				errors = append(errors, err.(*ValidationError))
			} else {
				matched = append(matched, i)
				if s.DraftVersion >= 2020 {
					delete(vd.uneval.items, i)
				}
			}
		}

		// minContains --
		if s.MinContains != nil {
			if len(matched) < *s.MinContains {
				vd.addErrors(errors, &kind.MinContains{Got: matched, Want: *s.MinContains})
			}
		} else if len(matched) == 0 {
			vd.addErrors(errors, &kind.Contains{})
		}

		// maxContains --
		if s.MaxContains != nil {
			if len(matched) > *s.MaxContains {
				vd.addError(&kind.MaxContains{Got: matched, Want: *s.MaxContains})
			}
		}
	}
}

func (vd *validator) strValidate(str string) {
	s := vd.sch

	strLen := -1
	if s.MinLength != nil || s.MaxLength != nil {
		strLen = utf8.RuneCount([]byte(str))
	}

	// minLength --
	if s.MinLength != nil {
		if strLen < *s.MinLength {
			vd.addError(&kind.MinLength{Got: strLen, Want: *s.MinLength})
		}
	}

	// maxLength --
	if s.MaxLength != nil {
		if strLen > *s.MaxLength {
			vd.addError(&kind.MaxLength{Got: strLen, Want: *s.MaxLength})
		}
	}

	// pattern --
	if s.Pattern != nil {
		if !s.Pattern.MatchString(str) {
			vd.addError(&kind.Pattern{Got: str, Want: s.Pattern.String()})
		}
	}

	if s.DraftVersion == 6 {
		return
	}

	var err error

	// contentEncoding --
	decoded := []byte(str)
	if s.ContentEncoding != nil {
		decoded, err = s.ContentEncoding.Decode(str)
		if err != nil {
			decoded = nil
			vd.addError(&kind.ContentEncoding{Want: s.ContentEncoding.Name, Err: err})
		}
	}

	var deserialized *any
	if decoded != nil && s.ContentMediaType != nil {
		if s.ContentSchema == nil {
			err = s.ContentMediaType.Validate(decoded)
		} else {
			var value any
			value, err = s.ContentMediaType.UnmarshalJSON(decoded)
			if err == nil {
				deserialized = &value
			}
		}
		if err != nil {
			vd.addError(&kind.ContentMediaType{
				Got:  decoded,
				Want: s.ContentMediaType.Name,
				Err:  err,
			})
		}
	}

	if deserialized != nil && s.ContentSchema != nil {
		sch, meta, resources := s.ContentSchema, vd.meta, vd.resources
		res := vd.metaResource(sch)
		if res != nil {
			meta = res.dialect.getSchema(vd.assertVocabs, vd.vocabularies)
			sch = meta
		}
		if err = sch.validate(*deserialized, vd.regexpEngine, meta, resources, vd.assertVocabs, vd.vocabularies); err != nil {
			verr := err.(*ValidationError)
			verr.SchemaURL = s.Location
			verr.ErrorKind = &kind.ContentSchema{}
			vd.addErr(verr)
		}
	}
}

func (vd *validator) numValidate(v any) {
	s := vd.sch

	var numVal *big.Rat
	num := func() *big.Rat {
		if numVal == nil {
			numVal, _ = new(big.Rat).SetString(fmt.Sprintf("%v", v))
		}
		return numVal
	}

	// minimum --
	if s.Minimum != nil && num().Cmp(s.Minimum) < 0 {
		vd.addError(&kind.Minimum{Got: num(), Want: s.Minimum})
	}

	// maximum --
	if s.Maximum != nil && num().Cmp(s.Maximum) > 0 {
		vd.addError(&kind.Maximum{Got: num(), Want: s.Maximum})
	}

	// exclusiveMinimum
	if s.ExclusiveMinimum != nil && num().Cmp(s.ExclusiveMinimum) <= 0 {
		vd.addError(&kind.ExclusiveMinimum{Got: num(), Want: s.ExclusiveMinimum})
	}

	// exclusiveMaximum
	if s.ExclusiveMaximum != nil && num().Cmp(s.ExclusiveMaximum) >= 0 {
		vd.addError(&kind.ExclusiveMaximum{Got: num(), Want: s.ExclusiveMaximum})
	}

	// multipleOf
	if s.MultipleOf != nil {
		if q := new(big.Rat).Quo(num(), s.MultipleOf); !q.IsInt() {
			vd.addError(&kind.MultipleOf{Got: num(), Want: s.MultipleOf})
		}
	}
}

func (vd *validator) condValidate() {
	s := vd.sch

	// not --
	if s.Not != nil {
		if vd.validateSelf(s.Not, "", true) == nil {
			vd.addError(&kind.Not{})
		}
	}

	// allOf --
	if len(s.AllOf) > 0 {
		var errors []*ValidationError
		for _, sch := range s.AllOf {
			if err := vd.validateSelf(sch, "", false); err != nil {
				errors = append(errors, err.(*ValidationError))
				if vd.boolResult {
					break
				}
			}
		}
		if len(errors) != 0 {
			vd.addErrors(errors, &kind.AllOf{})
		}
	}

	// anyOf
	if len(s.AnyOf) > 0 {
		var matched bool
		var errors []*ValidationError
		for _, sch := range s.AnyOf {
			if err := vd.validateSelf(sch, "", false); err != nil {
				errors = append(errors, err.(*ValidationError))
			} else {
				matched = true
				// for uneval, all schemas must be evaluated
				if vd.uneval.isEmpty() {
					break
				}
			}
		}
		if !matched {
			vd.addErrors(errors, &kind.AnyOf{})
		}
	}

	// oneOf
	if len(s.OneOf) > 0 {
		var matched = -1
		var errors []*ValidationError
		for i, sch := range s.OneOf {
			if err := vd.validateSelf(sch, "", matched != -1); err != nil {
				if matched == -1 {
					errors = append(errors, err.(*ValidationError))
				}
			} else {
				if matched == -1 {
					matched = i
				} else {
					vd.addError(&kind.OneOf{Subschemas: []int{matched, i}})
					break
				}
			}
		}
		if matched == -1 {
			vd.addErrors(errors, &kind.OneOf{Subschemas: nil})
		}
	}

	// if, then, else --
	if s.If != nil {
		if vd.validateSelf(s.If, "", true) == nil {
			if s.Then != nil {
				vd.addErr(vd.validateSelf(s.Then, "", false))
			}
		} else if s.Else != nil {
			vd.addErr(vd.validateSelf(s.Else, "", false))
		}
	}
}

func (vd *validator) unevalValidate() {
	s := vd.sch

	// unevaluatedProperties
	if obj, ok := vd.v.(map[string]any); ok && s.UnevaluatedProperties != nil {
		for pname := range vd.uneval.props {
			if pvalue, ok := obj[pname]; ok {
				vd.addErr(vd.validateVal(s.UnevaluatedProperties, pvalue, pname))
			}
		}
		vd.uneval.props = nil
	}

	// unevaluatedItems
	if arr, ok := vd.v.([]any); ok && s.UnevaluatedItems != nil {
		for i := range vd.uneval.items {
			vd.addErr(vd.validateVal(s.UnevaluatedItems, arr[i], strconv.Itoa(i)))
		}
		vd.uneval.items = nil
	}
}

// validation helpers --

func (vd *validator) validateSelf(sch *Schema, refKw string, boolResult bool) error {
	scp := vd.scp.child(sch, refKw, vd.scp.vid)
	uneval := unevalFrom(vd.v, sch, !vd.uneval.isEmpty())
	subvd := validator{
		v:            vd.v,
		vloc:         vd.vloc,
		sch:          sch,
		scp:          scp,
		uneval:       uneval,
		errors:       nil,
		boolResult:   vd.boolResult || boolResult,
		regexpEngine: vd.regexpEngine,
		meta:         vd.meta,
		resources:    vd.resources,
		assertVocabs: vd.assertVocabs,
		vocabularies: vd.vocabularies,
	}
	subvd.handleMeta()
	uneval, err := subvd.validate()
	if err == nil {
		vd.uneval.merge(uneval)
	}
	return err
}

func (vd *validator) validateVal(sch *Schema, v any, vtok string) error {
	vloc := append(vd.vloc, vtok)
	scp := vd.scp.child(sch, "", vd.scp.vid+1)
	uneval := unevalFrom(v, sch, false)
	subvd := validator{
		v:            v,
		vloc:         vloc,
		sch:          sch,
		scp:          scp,
		uneval:       uneval,
		errors:       nil,
		boolResult:   vd.boolResult,
		regexpEngine: vd.regexpEngine,
		meta:         vd.meta,
		resources:    vd.resources,
		assertVocabs: vd.assertVocabs,
		vocabularies: vd.vocabularies,
	}
	subvd.handleMeta()
	_, err := subvd.validate()
	return err
}

func (vd *validator) validateValue(sch *Schema, v any, vpath []string) error {
	vloc := append(vd.vloc, vpath...)
	scp := vd.scp.child(sch, "", vd.scp.vid+1)
	uneval := unevalFrom(v, sch, false)
	subvd := validator{
		v:            v,
		vloc:         vloc,
		sch:          sch,
		scp:          scp,
		uneval:       uneval,
		errors:       nil,
		boolResult:   vd.boolResult,
		regexpEngine: vd.regexpEngine,
		meta:         vd.meta,
		resources:    vd.resources,
		assertVocabs: vd.assertVocabs,
		vocabularies: vd.vocabularies,
	}
	subvd.handleMeta()
	_, err := subvd.validate()
	return err
}

func (vd *validator) metaResource(sch *Schema) *resource {
	if sch != vd.meta {
		return nil
	}
	ptr := ""
	for _, tok := range vd.instanceLocation() {
		ptr += "/"
		ptr += escape(tok)
	}
	return vd.resources[jsonPointer(ptr)]
}

func (vd *validator) handleMeta() {
	res := vd.metaResource(vd.sch)
	if res == nil {
		return
	}
	sch := res.dialect.getSchema(vd.assertVocabs, vd.vocabularies)
	vd.meta = sch
	vd.sch = sch
}

// reference validation --

func (vd *validator) validateRef(sch *Schema, kw string) error {
	err := vd.validateSelf(sch, kw, false)
	if err != nil {
		refErr := vd.error(&kind.Reference{Keyword: kw, URL: sch.Location})
		verr := err.(*ValidationError)
		if _, ok := verr.ErrorKind.(*kind.Group); ok {
			refErr.Causes = verr.Causes
		} else {
			refErr.Causes = append(refErr.Causes, verr)
		}
		return refErr
	}
	return nil
}

func (vd *validator) resolveRecursiveAnchor(fallback *Schema) *Schema {
	sch := fallback
	scp := vd.scp
	for scp != nil {
		if scp.sch.resource.RecursiveAnchor {
			sch = scp.sch
		}
		scp = scp.parent
	}
	return sch
}

func (vd *validator) resolveDynamicAnchor(name string, fallback *Schema) *Schema {
	sch := fallback
	scp := vd.scp
	for scp != nil {
		if dsch, ok := scp.sch.resource.dynamicAnchors[name]; ok {
			sch = dsch
		}
		scp = scp.parent
	}
	return sch
}

func (vd *validator) validateRefs() {
	// $recursiveRef --
	if sch := vd.sch.RecursiveRef; sch != nil {
		if sch.RecursiveAnchor {
			sch = vd.resolveRecursiveAnchor(sch)
		}
		vd.addErr(vd.validateRef(sch, "$recursiveRef"))
	}

	// $dynamicRef --
	if dref := vd.sch.DynamicRef; dref != nil {
		sch := dref.Ref // initial target
		if dref.Anchor != "" {
			// $dynamicRef includes anchor
			if sch.DynamicAnchor == dref.Anchor {
				// initial target has matching $dynamicAnchor
				sch = vd.resolveDynamicAnchor(dref.Anchor, sch)
			}
		}
		vd.addErr(vd.validateRef(sch, "$dynamicRef"))
	}
}

// error helpers --

func (vd *validator) instanceLocation() []string {
	return slices.Clone(vd.vloc)
}

func (vd *validator) error(kind ErrorKind) *ValidationError {
	if vd.boolResult {
		return &ValidationError{}
	}
	return &ValidationError{
		SchemaURL:        vd.sch.Location,
		InstanceLocation: vd.instanceLocation(),
		ErrorKind:        kind,
		Causes:           nil,
	}
}

func (vd *validator) addErr(err error) {
	if err != nil {
		vd.errors = append(vd.errors, err.(*ValidationError))
	}
}

func (vd *validator) addError(kind ErrorKind) {
	vd.errors = append(vd.errors, vd.error(kind))
}

func (vd *validator) addErrors(errors []*ValidationError, kind ErrorKind) {
	err := vd.error(kind)
	err.Causes = errors
	vd.errors = append(vd.errors, err)
}

func (vd *validator) findMissing(obj map[string]any, reqd []string) []string {
	var missing []string
	for _, pname := range reqd {
		if _, ok := obj[pname]; !ok {
			if vd.boolResult {
				return []string{} // non-nil
			}
			missing = append(missing, pname)
		}
	}
	return missing
}

// --

type scope struct {
	sch *Schema

	// if empty, compute from self.sch and self.parent.sch.
	// not empty, only when there is a jump i.e, $ref, $XXXRef
	refKeyword string

	// unique id of value being validated
	// if two scopes validate same value, they will have
	// same vid
	vid int

	parent *scope
}

func (sc *scope) child(sch *Schema, refKeyword string, vid int) *scope {
	return &scope{sch, refKeyword, vid, sc}
}

func (sc *scope) checkCycle() *scope {
	scp := sc.parent
	for scp != nil {
		if scp.vid != sc.vid {
			break
		}
		if scp.sch == sc.sch {
			return scp
		}
		scp = scp.parent
	}
	return nil
}

func (sc *scope) kwLoc() string {
	var loc string
	for sc.parent != nil {
		if sc.refKeyword != "" {
			loc = fmt.Sprintf("/%s%s", escape(sc.refKeyword), loc)
		} else {
			cur := sc.sch.Location
			parent := sc.parent.sch.Location
			loc = fmt.Sprintf("%s%s", cur[len(parent):], loc)
		}
		sc = sc.parent
	}
	return loc
}

// --

type uneval struct {
	props map[string]struct{}
	items map[int]struct{}
}

func unevalFrom(v any, sch *Schema, callerNeeds bool) *uneval {
	uneval := &uneval{}
	switch v := v.(type) {
	case map[string]any:
		if !sch.allPropsEvaluated && (callerNeeds || sch.UnevaluatedProperties != nil) {
			uneval.props = map[string]struct{}{}
			for k := range v {
				uneval.props[k] = struct{}{}
			}
		}
	case []any:
		if !sch.allItemsEvaluated && (callerNeeds || sch.UnevaluatedItems != nil) && sch.numItemsEvaluated < len(v) {
			uneval.items = map[int]struct{}{}
			for i := sch.numItemsEvaluated; i < len(v); i++ {
				uneval.items[i] = struct{}{}
			}
		}
	}
	return uneval
}

func (ue *uneval) merge(other *uneval) {
	for k := range ue.props {
		if _, ok := other.props[k]; !ok {
			delete(ue.props, k)
		}
	}
	for i := range ue.items {
		if _, ok := other.items[i]; !ok {
			delete(ue.items, i)
		}
	}
}

func (ue *uneval) isEmpty() bool {
	return len(ue.props) == 0 && len(ue.items) == 0
}

// --

type ValidationError struct {
	// absolute, dereferenced schema location.
	SchemaURL string

	// location of the JSON value within the instance being validated.
	InstanceLocation []string

	// kind of error
	ErrorKind ErrorKind

	// holds nested errors
	Causes []*ValidationError
}

type ErrorKind interface {
	KeywordPath() []string
	LocalizedString(*message.Printer) string
}
