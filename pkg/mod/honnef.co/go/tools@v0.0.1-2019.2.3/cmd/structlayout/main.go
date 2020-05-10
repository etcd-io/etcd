// structlayout displays the layout (field sizes and padding) of structs.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"go/build"
	"go/types"
	"log"
	"os"

	"honnef.co/go/tools/gcsizes"
	st "honnef.co/go/tools/structlayout"
	"honnef.co/go/tools/version"

	"golang.org/x/tools/go/loader"
)

var (
	fJSON    bool
	fVersion bool
)

func init() {
	flag.BoolVar(&fJSON, "json", false, "Format data as JSON")
	flag.BoolVar(&fVersion, "version", false, "Print version and exit")
}

func main() {
	log.SetFlags(0)
	flag.Parse()

	if fVersion {
		version.Print()
		os.Exit(0)
	}

	if len(flag.Args()) != 2 {
		flag.Usage()
		os.Exit(1)
	}

	conf := loader.Config{
		Build: &build.Default,
	}

	var pkg string
	var typName string
	pkg = flag.Args()[0]
	typName = flag.Args()[1]
	conf.Import(pkg)

	lprog, err := conf.Load()
	if err != nil {
		log.Fatal(err)
	}
	var typ types.Type
	obj := lprog.Package(pkg).Pkg.Scope().Lookup(typName)
	if obj == nil {
		log.Fatal("couldn't find type")
	}
	typ = obj.Type()

	st, ok := typ.Underlying().(*types.Struct)
	if !ok {
		log.Fatal("identifier is not a struct type")
	}

	fields := sizes(st, typ.(*types.Named).Obj().Name(), 0, nil)
	if fJSON {
		emitJSON(fields)
	} else {
		emitText(fields)
	}
}

func emitJSON(fields []st.Field) {
	if fields == nil {
		fields = []st.Field{}
	}
	json.NewEncoder(os.Stdout).Encode(fields)
}

func emitText(fields []st.Field) {
	for _, field := range fields {
		fmt.Println(field)
	}
}
func sizes(typ *types.Struct, prefix string, base int64, out []st.Field) []st.Field {
	s := gcsizes.ForArch(build.Default.GOARCH)
	n := typ.NumFields()
	var fields []*types.Var
	for i := 0; i < n; i++ {
		fields = append(fields, typ.Field(i))
	}
	offsets := s.Offsetsof(fields)
	for i := range offsets {
		offsets[i] += base
	}

	pos := base
	for i, field := range fields {
		if offsets[i] > pos {
			padding := offsets[i] - pos
			out = append(out, st.Field{
				IsPadding: true,
				Start:     pos,
				End:       pos + padding,
				Size:      padding,
			})
			pos += padding
		}
		size := s.Sizeof(field.Type())
		if typ2, ok := field.Type().Underlying().(*types.Struct); ok && typ2.NumFields() != 0 {
			out = sizes(typ2, prefix+"."+field.Name(), pos, out)
		} else {
			out = append(out, st.Field{
				Name:  prefix + "." + field.Name(),
				Type:  field.Type().String(),
				Start: offsets[i],
				End:   offsets[i] + size,
				Size:  size,
				Align: s.Alignof(field.Type()),
			})
		}
		pos += size
	}

	if len(out) == 0 {
		return out
	}
	field := &out[len(out)-1]
	if field.Size == 0 {
		field.Size = 1
		field.End++
	}
	pad := s.Sizeof(typ) - field.End
	if pad > 0 {
		out = append(out, st.Field{
			IsPadding: true,
			Start:     field.End,
			End:       field.End + pad,
			Size:      pad,
		})
	}

	return out
}
