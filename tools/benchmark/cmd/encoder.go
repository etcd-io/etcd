// Copyright 2020 The etcd Authors
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

package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

type encoder interface {
	mustWrite(v interface{})
}

type jsonEncoder struct {
	enc *json.Encoder
}

func newJsonEncoder(writer io.Writer) encoder {
	return jsonEncoder{
		enc: json.NewEncoder(writer),
	}
}

func (e jsonEncoder) mustWrite(v interface{}) {
	if err := e.enc.Encode(v); err != nil {
		panic(err)
	}
}

// shouldEncode suppresses stdout if output format is set and valid, and
// returns a restore function for recovering stdout.
func shouldEncode(format string) (encoder, func(), error) {
	if len(format) == 0 {
		return nil, func() {}, nil
	}

	stdout := os.Stdout
	restore := func() { os.Stdout = stdout }

	var err error
	if os.Stdout, err = os.Open(os.DevNull); err != nil {
		restore()
		return nil, nil, err
	}

	switch format {
	case "json":
		return newJsonEncoder(stdout), restore, nil
	}

	restore()
	return nil, nil, fmt.Errorf("unsupported format %v", format)
}
