package render

//go:generate esc -o bindata/esc.go -pkg=bindata templates
import (
	"fmt"
	"io"
	"io/ioutil"
	"path"
	"strings"
	"text/template"

	"github.com/cweill/gotests/internal/models"
	"github.com/cweill/gotests/internal/render/bindata"
)

const name = "name"

var (
	tmpls *template.Template
)

func init() {
	initEmptyTmpls()
	for _, name := range bindata.AssetNames() {
		tmpls = template.Must(tmpls.Parse(bindata.FSMustString(false, name)))
	}
}

// LoadCustomTemplates allows to load in custom templates from a specified path.
func LoadCustomTemplates(dir string) error {
	initEmptyTmpls()

	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("ioutil.ReadDir: %v", err)
	}

	templateFiles := []string{}
	for _, f := range files {
		templateFiles = append(templateFiles, path.Join(dir, f.Name()))
	}
	tmpls, err = tmpls.ParseFiles(templateFiles...)
	if err != nil {
		return fmt.Errorf("tmpls.ParseFiles: %v", err)
	}
	return nil
}

func initEmptyTmpls() {
	tmpls = template.New("render").Funcs(map[string]interface{}{
		"Field":    fieldName,
		"Receiver": receiverName,
		"Param":    parameterName,
		"Want":     wantName,
		"Got":      gotName,
	})
}

func fieldName(f *models.Field) string {
	var n string
	if f.IsNamed() {
		n = f.Name
	} else {
		n = f.Type.String()
	}
	return n
}

func receiverName(f *models.Receiver) string {
	var n string
	if f.IsNamed() {
		n = f.Name
	} else {
		n = f.ShortName()
	}
	if n == "name" {
		// Avoid conflict with test struct's "name" field.
		n = "n"
	}
	return n
}

func parameterName(f *models.Field) string {
	var n string
	if f.IsNamed() {
		n = f.Name
	} else {
		n = fmt.Sprintf("in%v", f.Index)
	}
	return n
}

func wantName(f *models.Field) string {
	var n string
	if f.IsNamed() {
		n = "want" + strings.Title(f.Name)
	} else if f.Index == 0 {
		n = "want"
	} else {
		n = fmt.Sprintf("want%v", f.Index)
	}
	return n
}

func gotName(f *models.Field) string {
	var n string
	if f.IsNamed() {
		n = "got" + strings.Title(f.Name)
	} else if f.Index == 0 {
		n = "got"
	} else {
		n = fmt.Sprintf("got%v", f.Index)
	}
	return n
}

func Header(w io.Writer, h *models.Header) error {
	if err := tmpls.ExecuteTemplate(w, "header", h); err != nil {
		return err
	}
	_, err := w.Write(h.Code)
	return err
}

func TestFunction(w io.Writer, f *models.Function, printInputs bool, subtests bool) error {
	return tmpls.ExecuteTemplate(w, "function", struct {
		*models.Function
		PrintInputs bool
		Subtests    bool
	}{
		Function:    f,
		PrintInputs: printInputs,
		Subtests:    subtests,
	})
}
