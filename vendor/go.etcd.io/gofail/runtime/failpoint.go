// Copyright 2016 CoreOS, Inc.
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

package runtime

import (
	"fmt"
	"sync"
)

type Failpoint struct {
	t   *terms
	mux sync.RWMutex
}

func NewFailpoint(name string) *Failpoint {
	return register(name)
}

// Acquire gets evalutes the failpoint terms; if the failpoint
// is active, it will return a value. Otherwise, returns a non-nil error.
//
// Notice that during the exection of Acquire(), the failpoint can be disabled,
// but the already in-flight execution won't be terminated
func (fp *Failpoint) Acquire() (interface{}, error) {
	fp.mux.RLock()
	// terms are locked during execution, so deepcopy is not required as no change can be made during execution
	cachedT := fp.t
	fp.mux.RUnlock()

	if cachedT == nil {
		return nil, ErrDisabled
	}
	result := cachedT.eval()
	if result == nil {
		return nil, ErrDisabled
	}
	return result, nil
}

// BadType is called when the failpoint evaluates to the wrong type.
func (fp *Failpoint) BadType(v interface{}, t string) {
	fmt.Printf("failpoint: %q got value %v of type \"%T\" but expected type %q\n", fp.t.fpath, v, v, t)
}

func (fp *Failpoint) SetTerm(t *terms) {
	fp.mux.Lock()
	defer fp.mux.Unlock()

	fp.t = t
}

func (fp *Failpoint) ClearTerm() error {
	fp.mux.Lock()
	defer fp.mux.Unlock()

	if fp.t == nil {
		return ErrDisabled
	}
	fp.t = nil

	return nil
}

func (fp *Failpoint) Status() (string, int, error) {
	fp.mux.RLock()
	defer fp.mux.RUnlock()

	t := fp.t
	if t == nil {
		return "", 0, ErrDisabled
	}

	return t.desc, t.counter, nil
}
