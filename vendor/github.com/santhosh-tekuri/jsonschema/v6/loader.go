package jsonschema

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	gourl "net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// URLLoader knows how to load json from given url.
type URLLoader interface {
	// Load loads json from given absolute url.
	Load(url string) (any, error)
}

// --

// FileLoader loads json file url.
type FileLoader struct{}

func (l FileLoader) Load(url string) (any, error) {
	path, err := l.ToFile(url)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return UnmarshalJSON(f)
}

// ToFile is helper method to convert file url to file path.
func (l FileLoader) ToFile(url string) (string, error) {
	u, err := gourl.Parse(url)
	if err != nil {
		return "", err
	}
	if u.Scheme != "file" {
		return "", fmt.Errorf("invalid file url: %s", u)
	}
	path := u.Path
	if runtime.GOOS == "windows" {
		path = strings.TrimPrefix(path, "/")
		path = filepath.FromSlash(path)
	}
	return path, nil
}

// --

// SchemeURLLoader delegates to other [URLLoaders]
// based on url scheme.
type SchemeURLLoader map[string]URLLoader

func (l SchemeURLLoader) Load(url string) (any, error) {
	u, err := gourl.Parse(url)
	if err != nil {
		return nil, err
	}
	ll, ok := l[u.Scheme]
	if !ok {
		return nil, &UnsupportedURLSchemeError{u.String()}
	}
	return ll.Load(url)
}

// --

//go:embed metaschemas
var metaFS embed.FS

func openMeta(url string) (fs.File, error) {
	u, meta := strings.CutPrefix(url, "http://json-schema.org/")
	if !meta {
		u, meta = strings.CutPrefix(url, "https://json-schema.org/")
	}
	if meta {
		if u == "schema" {
			return openMeta(draftLatest.url)
		}
		f, err := metaFS.Open("metaschemas/" + u)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil, nil
			}
			return nil, err
		}
		return f, err
	}
	return nil, nil

}

func isMeta(url string) bool {
	f, err := openMeta(url)
	if err != nil {
		return true
	}
	if f != nil {
		f.Close()
		return true
	}
	return false
}

func loadMeta(url string) (any, error) {
	f, err := openMeta(url)
	if err != nil {
		return nil, err
	}
	if f == nil {
		return nil, nil
	}
	defer f.Close()
	return UnmarshalJSON(f)
}

// --

type defaultLoader struct {
	docs   map[url]any // docs loaded so far
	loader URLLoader
}

func (l *defaultLoader) add(url url, doc any) bool {
	if _, ok := l.docs[url]; ok {
		return false
	}
	l.docs[url] = doc
	return true
}

func (l *defaultLoader) load(url url) (any, error) {
	if doc, ok := l.docs[url]; ok {
		return doc, nil
	}
	doc, err := loadMeta(url.String())
	if err != nil {
		return nil, err
	}
	if doc != nil {
		l.add(url, doc)
		return doc, nil
	}
	if l.loader == nil {
		return nil, &LoadURLError{url.String(), errors.New("no URLLoader set")}
	}
	doc, err = l.loader.Load(url.String())
	if err != nil {
		return nil, &LoadURLError{URL: url.String(), Err: err}
	}
	l.add(url, doc)
	return doc, nil
}

func (l *defaultLoader) getDraft(up urlPtr, doc any, defaultDraft *Draft, cycle map[url]struct{}) (*Draft, error) {
	obj, ok := doc.(map[string]any)
	if !ok {
		return defaultDraft, nil
	}
	sch, ok := strVal(obj, "$schema")
	if !ok {
		return defaultDraft, nil
	}
	if draft := draftFromURL(sch); draft != nil {
		return draft, nil
	}
	sch, _ = split(sch)
	if _, err := gourl.Parse(sch); err != nil {
		return nil, &InvalidMetaSchemaURLError{up.String(), err}
	}
	schUrl := url(sch)
	if up.ptr.isEmpty() && schUrl == up.url {
		return nil, &UnsupportedDraftError{schUrl.String()}
	}
	if _, ok := cycle[schUrl]; ok {
		return nil, &MetaSchemaCycleError{schUrl.String()}
	}
	cycle[schUrl] = struct{}{}
	doc, err := l.load(schUrl)
	if err != nil {
		return nil, err
	}
	return l.getDraft(urlPtr{schUrl, ""}, doc, defaultDraft, cycle)
}

func (l *defaultLoader) getMetaVocabs(doc any, draft *Draft, vocabularies map[string]*Vocabulary) ([]string, error) {
	obj, ok := doc.(map[string]any)
	if !ok {
		return nil, nil
	}
	sch, ok := strVal(obj, "$schema")
	if !ok {
		return nil, nil
	}
	if draft := draftFromURL(sch); draft != nil {
		return nil, nil
	}
	sch, _ = split(sch)
	if _, err := gourl.Parse(sch); err != nil {
		return nil, &ParseURLError{sch, err}
	}
	schUrl := url(sch)
	doc, err := l.load(schUrl)
	if err != nil {
		return nil, err
	}
	return draft.getVocabs(schUrl, doc, vocabularies)
}

// --

type LoadURLError struct {
	URL string
	Err error
}

func (e *LoadURLError) Error() string {
	return fmt.Sprintf("failing loading %q: %v", e.URL, e.Err)
}

// --

type UnsupportedURLSchemeError struct {
	url string
}

func (e *UnsupportedURLSchemeError) Error() string {
	return fmt.Sprintf("no URLLoader registered for %q", e.url)
}

// --

type ResourceExistsError struct {
	url string
}

func (e *ResourceExistsError) Error() string {
	return fmt.Sprintf("resource for %q already exists", e.url)
}

// --

// UnmarshalJSON unmarshals into [any] without losing
// number precision using [json.Number].
func UnmarshalJSON(r io.Reader) (any, error) {
	decoder := json.NewDecoder(r)
	decoder.UseNumber()
	var doc any
	if err := decoder.Decode(&doc); err != nil {
		return nil, err
	}
	if _, err := decoder.Token(); err == nil || err != io.EOF {
		return nil, fmt.Errorf("invalid character after top-level value")
	}
	return doc, nil
}
