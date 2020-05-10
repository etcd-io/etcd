// structlayout-optimize reorders struct fields to minimize the amount
// of padding.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"

	st "honnef.co/go/tools/structlayout"
	"honnef.co/go/tools/version"
)

var (
	fJSON    bool
	fRecurse bool
	fVersion bool
)

func init() {
	flag.BoolVar(&fJSON, "json", false, "Format data as JSON")
	flag.BoolVar(&fRecurse, "r", false, "Break up structs and reorder their fields freely")
	flag.BoolVar(&fVersion, "version", false, "Print version and exit")
}

func main() {
	log.SetFlags(0)
	flag.Parse()

	if fVersion {
		version.Print()
		os.Exit(0)
	}

	var in []st.Field
	if err := json.NewDecoder(os.Stdin).Decode(&in); err != nil {
		log.Fatal(err)
	}
	if len(in) == 0 {
		return
	}
	if !fRecurse {
		in = combine(in)
	}
	var fields []st.Field
	for _, field := range in {
		if field.IsPadding {
			continue
		}
		fields = append(fields, field)
	}
	optimize(fields)
	fields = pad(fields)

	if fJSON {
		json.NewEncoder(os.Stdout).Encode(fields)
	} else {
		for _, field := range fields {
			fmt.Println(field)
		}
	}
}

func combine(fields []st.Field) []st.Field {
	new := st.Field{}
	cur := ""
	var out []st.Field
	wasPad := true
	for _, field := range fields {
		var prefix string
		if field.IsPadding {
			wasPad = true
			continue
		}
		p := strings.Split(field.Name, ".")
		prefix = strings.Join(p[:2], ".")
		if field.Align > new.Align {
			new.Align = field.Align
		}
		if !wasPad {
			new.End = field.Start
			new.Size = new.End - new.Start
		}
		if prefix != cur {
			if cur != "" {
				out = append(out, new)
			}
			cur = prefix
			new = field
			new.Name = prefix
		} else {
			new.Type = "struct"
		}
		wasPad = false
	}
	new.Size = new.End - new.Start
	out = append(out, new)
	return out
}

func optimize(fields []st.Field) {
	sort.Sort(&byAlignAndSize{fields})
}

func pad(fields []st.Field) []st.Field {
	if len(fields) == 0 {
		return nil
	}
	var out []st.Field
	pos := int64(0)
	offsets := offsetsof(fields)
	alignment := int64(1)
	for i, field := range fields {
		if field.Align > alignment {
			alignment = field.Align
		}
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
		field.Start = pos
		field.End = pos + field.Size
		out = append(out, field)
		pos += field.Size
	}
	sz := size(out)
	pad := align(sz, alignment) - sz
	if pad > 0 {
		field := out[len(out)-1]
		out = append(out, st.Field{
			IsPadding: true,
			Start:     field.End,
			End:       field.End + pad,
			Size:      pad,
		})
	}
	return out
}

func size(fields []st.Field) int64 {
	n := int64(0)
	for _, field := range fields {
		n += field.Size
	}
	return n
}

type byAlignAndSize struct {
	fields []st.Field
}

func (s *byAlignAndSize) Len() int { return len(s.fields) }
func (s *byAlignAndSize) Swap(i, j int) {
	s.fields[i], s.fields[j] = s.fields[j], s.fields[i]
}

func (s *byAlignAndSize) Less(i, j int) bool {
	// Place zero sized objects before non-zero sized objects.
	if s.fields[i].Size == 0 && s.fields[j].Size != 0 {
		return true
	}
	if s.fields[j].Size == 0 && s.fields[i].Size != 0 {
		return false
	}

	// Next, place more tightly aligned objects before less tightly aligned objects.
	if s.fields[i].Align != s.fields[j].Align {
		return s.fields[i].Align > s.fields[j].Align
	}

	// Lastly, order by size.
	if s.fields[i].Size != s.fields[j].Size {
		return s.fields[i].Size > s.fields[j].Size
	}

	return false
}

func offsetsof(fields []st.Field) []int64 {
	offsets := make([]int64, len(fields))
	var o int64
	for i, f := range fields {
		a := f.Align
		o = align(o, a)
		offsets[i] = o
		o += f.Size
	}
	return offsets
}

// align returns the smallest y >= x such that y % a == 0.
func align(x, a int64) int64 {
	y := x + a - 1
	return y - y%a
}
