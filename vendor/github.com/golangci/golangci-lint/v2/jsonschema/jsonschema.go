package jsonschema

import (
	"embed"
	"path/filepath"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

const (
	V1Schema   = "/golangci.v1.jsonschema.json"
	NextSchema = "/golangci.next.jsonschema.json"
)

//go:embed golangci.next.jsonschema.json golangci.v1.jsonschema.json
var content embed.FS

type EmbedLoader struct {
	jsonschema.FileLoader
}

func NewEmbedLoader() *EmbedLoader {
	return &EmbedLoader{}
}

func (f *EmbedLoader) Load(uri string) (any, error) {
	p, err := f.ToFile(uri)
	if err != nil {
		return nil, err
	}

	file, err := content.Open(filepath.Base(p))
	if err != nil {
		return nil, err
	}

	defer func() { _ = file.Close() }()

	return jsonschema.UnmarshalJSON(file)
}
