package pb

import (
	"math/rand"
	"sync"
	"text/template"

	"github.com/fatih/color"
)

// ProgressBarTemplate that template string
type ProgressBarTemplate string

// New creates new bar from template
func (pbt ProgressBarTemplate) New(total int) *ProgressBar {
	return New(total).SetTemplate(pbt)
}

// Start64 create and start new bar with given int64 total value
func (pbt ProgressBarTemplate) Start64(total int64) *ProgressBar {
	return New64(total).SetTemplate(pbt).Start()
}

// Start create and start new bar with given int total value
func (pbt ProgressBarTemplate) Start(total int) *ProgressBar {
	return pbt.Start64(int64(total))
}

var templateCacheMu sync.Mutex
var templateCache = make(map[string]*template.Template)

var defaultTemplateFuncs = template.FuncMap{
	// colors
	"black":      color.New(color.FgBlack).SprintFunc(),
	"red":        color.New(color.FgRed).SprintFunc(),
	"green":      color.New(color.FgGreen).SprintFunc(),
	"yellow":     color.New(color.FgYellow).SprintFunc(),
	"blue":       color.New(color.FgBlue).SprintFunc(),
	"magenta":    color.New(color.FgMagenta).SprintFunc(),
	"cyan":       color.New(color.FgCyan).SprintFunc(),
	"white":      color.New(color.FgWhite).SprintFunc(),
	"resetcolor": color.New(color.Reset).SprintFunc(),
	"rndcolor":   rndcolor,
	"rnd":        rnd,
}

func getTemplate(tmpl string) (t *template.Template, err error) {
	templateCacheMu.Lock()
	defer templateCacheMu.Unlock()
	t = templateCache[tmpl]
	if t != nil {
		// found in cache
		return
	}
	t = template.New("")
	fillTemplateFuncs(t)
	_, err = t.Parse(tmpl)
	if err != nil {
		t = nil
		return
	}
	templateCache[tmpl] = t
	return
}

func fillTemplateFuncs(t *template.Template) {
	t.Funcs(defaultTemplateFuncs)
	emf := make(template.FuncMap)
	elementsM.Lock()
	for k, v := range elements {
		element := v
		emf[k] = func(state *State, args ...string) string { return element.ProgressElement(state, args...) }
	}
	elementsM.Unlock()
	t.Funcs(emf)
	return
}

func rndcolor(s string) string {
	c := rand.Intn(int(color.FgWhite-color.FgBlack)) + int(color.FgBlack)
	return color.New(color.Attribute(c)).Sprint(s)
}

func rnd(args ...string) string {
	if len(args) == 0 {
		return ""
	}
	return args[rand.Intn(len(args))]
}
