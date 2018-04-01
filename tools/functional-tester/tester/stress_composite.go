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

func (cs *compositeStresser) Pause() {
	var wg sync.WaitGroup
	wg.Add(len(cs.stressers))
	for i := range cs.stressers {
		go func(s Stresser) {
			defer wg.Done()
			s.Pause()
		}(cs.stressers[i])
	}
	wg.Wait()
}

func (cs *compositeStresser) Close() {
	var wg sync.WaitGroup
	wg.Add(len(cs.stressers))
	for i := range cs.stressers {
		go func(s Stresser) {
			defer wg.Done()
			s.Close()
		}(cs.stressers[i])
	}
	wg.Wait()
}

func (cs *compositeStresser) ModifiedKeys() (modifiedKey int64) {
	for _, stress := range cs.stressers {
		modifiedKey += stress.ModifiedKeys()
	}
	return modifiedKey
}

func (cs *compositeStresser) Checker() Checker {
	var chks []Checker
	for _, s := range cs.stressers {
		if chk := s.Checker(); chk != nil {
			chks = append(chks, chk)
		}
	}
	if len(chks) == 0 {
		return nil
	}
	return newCompositeChecker(chks)
}
