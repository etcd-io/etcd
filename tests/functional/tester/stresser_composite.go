// Copyright 2018 The etcd Authors
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

package tester

import "sync"

// compositeStresser implements a Stresser that runs a slice of
// stressing clients concurrently.
type compositeStresser struct {
	stressers []Stresser
}

func (cs *compositeStresser) Stress() error {
	for i, s := range cs.stressers {
		if err := s.Stress(); err != nil {
			for j := 0; j < i; j++ {
				cs.stressers[j].Close()
			}
			return err
		}
	}
	return nil
}

func (cs *compositeStresser) Pause() (ems map[string]int) {
	var emu sync.Mutex
	ems = make(map[string]int)
	var wg sync.WaitGroup
	wg.Add(len(cs.stressers))
	for i := range cs.stressers {
		go func(s Stresser) {
			defer wg.Done()
			errs := s.Pause()
			for k, v := range errs {
				emu.Lock()
				ems[k] += v
				emu.Unlock()
			}
		}(cs.stressers[i])
	}
	wg.Wait()
	return ems
}

func (cs *compositeStresser) Close() (ems map[string]int) {
	var emu sync.Mutex
	ems = make(map[string]int)
	var wg sync.WaitGroup
	wg.Add(len(cs.stressers))
	for i := range cs.stressers {
		go func(s Stresser) {
			defer wg.Done()
			errs := s.Close()
			for k, v := range errs {
				emu.Lock()
				ems[k] += v
				emu.Unlock()
			}
		}(cs.stressers[i])
	}
	wg.Wait()
	return ems
}

func (cs *compositeStresser) ModifiedKeys() (modifiedKey int64) {
	for _, stress := range cs.stressers {
		modifiedKey += stress.ModifiedKeys()
	}
	return modifiedKey
}
