package pkg

import (
	th "html/template"
	tt "text/template"
)

const tmpl1 = `{{.Name}} {{.LastName}`
const tmpl2 = `{{fn}}`

func fn() {
	tt.New("").Parse(tmpl1) // want `template`
	tt.New("").Parse(tmpl2)
	t1 := tt.New("")
	t1.Parse(tmpl1)
	th.New("").Parse(tmpl1) // want `template`
	th.New("").Parse(tmpl2)
	t2 := th.New("")
	t2.Parse(tmpl1)
	tt.New("").Delims("[[", "]]").Parse("{{abc-}}")
}
