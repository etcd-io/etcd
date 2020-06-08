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
	"context"
	"fmt"
	"sync"
)

type Failpoint struct {
	cmu      sync.RWMutex
	ctx      context.Context
	cancel   context.CancelFunc
	donec    chan struct{}
	released bool

	mu sync.RWMutex
	t  *terms
}

func NewFailpoint(pkg, name string, goFailGo bool) *Failpoint {
	fp := &Failpoint{}
	if goFailGo {
		fp.ctx, fp.cancel = context.WithCancel(context.Background())
		fp.donec = make(chan struct{})
	}
	register(pkg+"/"+name, fp)
	return fp
}

// Acquire gets evalutes the failpoint terms; if the failpoint
// is active, it will return a value. Otherwise, returns a non-nil error.
func (fp *Failpoint) Acquire() (interface{}, error) {
	fp.mu.RLock()
	if fp.t == nil {
		fp.mu.RUnlock()
		return nil, ErrDisabled
	}
	v, err := fp.t.eval()
	if v == nil {
		err = ErrDisabled
	}
	if err != nil {
		fp.mu.RUnlock()
	}
	return v, err
}

// Release is called when the failpoint exists.
func (fp *Failpoint) Release() {
	fp.cmu.RLock()
	ctx := fp.ctx
	donec := fp.donec
	fp.cmu.RUnlock()
	if ctx != nil && !fp.released {
		<-ctx.Done()
		select {
		case <-donec:
		default:
			close(donec)
		}
	}

	fp.mu.RUnlock()
}

// BadType is called when the failpoint evaluates to the wrong type.
func (fp *Failpoint) BadType(v interface{}, t string) {
	fmt.Printf("failpoint: %q got value %v of type \"%T\" but expected type %q\n", fp.t.fpath, v, v, t)
}
