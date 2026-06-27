package jsonschema

import (
	"strconv"
	"strings"
)

// Position tells possible tokens in json.
type Position interface {
	collect(v any, ptr jsonPointer) map[jsonPointer]any
}

// --

type AllProp struct{}

func (AllProp) collect(v any, ptr jsonPointer) map[jsonPointer]any {
	obj, ok := v.(map[string]any)
	if !ok {
		return nil
	}
	m := map[jsonPointer]any{}
	for pname, pvalue := range obj {
		m[ptr.append(pname)] = pvalue
	}
	return m
}

// --

type AllItem struct{}

func (AllItem) collect(v any, ptr jsonPointer) map[jsonPointer]any {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	m := map[jsonPointer]any{}
	for i, item := range arr {
		m[ptr.append(strconv.Itoa(i))] = item
	}
	return m
}

// --

type Prop string

func (p Prop) collect(v any, ptr jsonPointer) map[jsonPointer]any {
	obj, ok := v.(map[string]any)
	if !ok {
		return nil
	}
	pvalue, ok := obj[string(p)]
	if !ok {
		return nil
	}
	return map[jsonPointer]any{
		ptr.append(string(p)): pvalue,
	}
}

// --

type Item int

func (i Item) collect(v any, ptr jsonPointer) map[jsonPointer]any {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	if i < 0 || int(i) >= len(arr) {
		return nil
	}
	return map[jsonPointer]any{
		ptr.append(strconv.Itoa(int(i))): arr[int(i)],
	}
}

// --

// SchemaPath tells where to look for subschema inside keyword.
type SchemaPath []Position

func schemaPath(path string) SchemaPath {
	var sp SchemaPath
	for _, tok := range strings.Split(path, "/") {
		var pos Position
		switch tok {
		case "*":
			pos = AllProp{}
		case "[]":
			pos = AllItem{}
		default:
			if i, err := strconv.Atoi(tok); err == nil {
				pos = Item(i)
			} else {
				pos = Prop(tok)
			}
		}
		sp = append(sp, pos)
	}
	return sp
}

func (sp SchemaPath) collect(v any, ptr jsonPointer) map[jsonPointer]any {
	if len(sp) == 0 {
		return map[jsonPointer]any{
			ptr: v,
		}
	}
	p, sp := sp[0], sp[1:]
	m := p.collect(v, ptr)
	mm := map[jsonPointer]any{}
	for ptr, v := range m {
		m = sp.collect(v, ptr)
		for k, v := range m {
			mm[k] = v
		}
	}
	return mm
}

func (sp SchemaPath) String() string {
	var sb strings.Builder
	for _, pos := range sp {
		if sb.Len() != 0 {
			sb.WriteByte('/')
		}
		switch pos := pos.(type) {
		case AllProp:
			sb.WriteString("*")
		case AllItem:
			sb.WriteString("[]")
		case Prop:
			sb.WriteString(string(pos))
		case Item:
			sb.WriteString(strconv.Itoa(int(pos)))
		}
	}
	return sb.String()
}
