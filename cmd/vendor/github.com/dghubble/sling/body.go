package sling

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"

	goquery "github.com/google/go-querystring/query"
)

// BodyProvider provides Body content for http.Request attachment.
type BodyProvider interface {
	// ContentType returns the Content-Type of the body.
	ContentType() string
	// Body returns the io.Reader body.
	Body() (io.Reader, error)
}

// bodyProvider provides the wrapped body value as a Body for reqests.
type bodyProvider struct {
	body io.Reader
}

func (p bodyProvider) ContentType() string {
	return ""
}

func (p bodyProvider) Body() (io.Reader, error) {
	return p.body, nil
}

// jsonBodyProvider encodes a JSON tagged struct value as a Body for requests.
// See https://golang.org/pkg/encoding/json/#MarshalIndent for details.
type jsonBodyProvider struct {
	payload interface{}
}

func (p jsonBodyProvider) ContentType() string {
	return jsonContentType
}

func (p jsonBodyProvider) Body() (io.Reader, error) {
	buf := &bytes.Buffer{}
	err := json.NewEncoder(buf).Encode(p.payload)
	if err != nil {
		return nil, err
	}
	return buf, nil
}

// formBodyProvider encodes a url tagged struct value as Body for requests.
// See https://godoc.org/github.com/google/go-querystring/query for details.
type formBodyProvider struct {
	payload interface{}
}

func (p formBodyProvider) ContentType() string {
	return formContentType
}

func (p formBodyProvider) Body() (io.Reader, error) {
	values, err := goquery.Values(p.payload)
	if err != nil {
		return nil, err
	}
	return strings.NewReader(values.Encode()), nil
}
