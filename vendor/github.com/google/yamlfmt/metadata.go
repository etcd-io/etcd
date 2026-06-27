// Copyright 2024 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package yamlfmt

import (
	"errors"
	"fmt"
	"strings"
	"unicode"

	"github.com/google/yamlfmt/internal/collections"
)

const MetadataIdentifier = "!yamlfmt!"

type MetadataType string

const (
	MetadataIgnore MetadataType = "ignore"
)

func IsMetadataType(mdValueStr string) bool {
	mdTypes := collections.Set[MetadataType]{}
	mdTypes.Add(MetadataIgnore)
	return mdTypes.Contains(MetadataType(mdValueStr))
}

type Metadata struct {
	Type    MetadataType
	LineNum int
}

var (
	ErrMalformedMetadata    = errors.New("metadata: malformed string")
	ErrUnrecognizedMetadata = errors.New("metadata: unrecognized type")
)

type MetadataError struct {
	err     error
	path    string
	lineNum int
	lineStr string
}

func (e *MetadataError) Error() string {
	return fmt.Sprintf(
		"%v: %s:%d:%s",
		e.err,
		e.path,
		e.lineNum,
		e.lineStr,
	)
}

func (e *MetadataError) Unwrap() error {
	return e.err
}

func ReadMetadata(content []byte, path string) (collections.Set[Metadata], collections.Errors) {
	metadata := collections.Set[Metadata]{}
	mdErrs := collections.Errors{}
	// This could be `\r\n` but it won't affect the outcome of this operation.
	contentLines := strings.Split(string(content), "\n")
	for i, line := range contentLines {
		mdidIndex := strings.Index(line, MetadataIdentifier)
		if mdidIndex == -1 {
			continue
		}
		mdStr := scanMetadata(line, mdidIndex)
		mdComponents := strings.Split(mdStr, ":")
		if len(mdComponents) != 2 {
			mdErrs = append(mdErrs, &MetadataError{
				path:    path,
				lineNum: i + 1,
				err:     ErrMalformedMetadata,
				lineStr: line,
			})
			continue
		}
		if IsMetadataType(mdComponents[1]) {
			metadata.Add(Metadata{LineNum: i + 1, Type: MetadataType(mdComponents[1])})
		} else {
			mdErrs = append(mdErrs, &MetadataError{
				path:    path,
				lineNum: i + 1,
				err:     ErrUnrecognizedMetadata,
				lineStr: line,
			})
		}
	}
	return metadata, mdErrs
}

func scanMetadata(line string, index int) string {
	mdBytes := []byte{}
	i := index
	for i < len(line) && !unicode.IsSpace(rune(line[i])) {
		mdBytes = append(mdBytes, line[i])
		i++
	}
	return string(mdBytes)
}
