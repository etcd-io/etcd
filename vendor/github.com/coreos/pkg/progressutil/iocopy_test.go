// Copyright 2016 CoreOS Inc
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

package progressutil

import (
	"bytes"
	"fmt"
	"io"
	"testing"
	"time"
)

type fakeReader struct {
	input chan []byte
	done  bool
}

func (fr *fakeReader) Read(p []byte) (int, error) {
	if fr.done {
		return 0, io.EOF
	}
	i := copy(p, <-fr.input)
	return i, nil
}

func TestCopyOne(t *testing.T) {
	cpp := NewCopyProgressPrinter()
	cpp.pbp.printToTTYAlways = true

	sampleData := []byte("this is a test!")

	fr := &fakeReader{make(chan []byte, 1), false}
	fw := &bytes.Buffer{}

	out := &bytes.Buffer{}

	err := cpp.AddCopy(fr, "download", int64(len(sampleData)*10), fw)
	if err != nil {
		t.Errorf("%v\n", err)
	}

	doneChan := make(chan error)
	go func() {
		doneChan <- cpp.PrintAndWait(out, time.Millisecond*10, nil)
	}()

	time.Sleep(time.Millisecond * 15)

	for i := 0; i < 10; i++ {
		// Empty the buffer
		printedData := out.Bytes()
		sizeString := ByteUnitStr(int64(len(sampleData)*i)) + " / " + ByteUnitStr(int64(len(sampleData)*10))
		bar := renderExpectedBar(80, "download", float64(i)/10, sizeString)
		var expectedOutput string
		if i == 0 {
			expectedOutput = fmt.Sprintf("%s\n", bar)
		} else {
			expectedOutput = fmt.Sprintf("\033[1A%s\n", bar)
		}
		if string(printedData) != expectedOutput {
			t.Errorf("unexpected output:\nexpected:\n\n%sactual:\n\n%s", expectedOutput, string(printedData))
		}
		if i == 9 {
			fr.done = true
		}
		fr.input <- sampleData

		out.Reset()

		time.Sleep(time.Millisecond * 10)
	}

	err = <-doneChan
	if err != nil {
		t.Errorf("error from PrintAndWait: %v", err)
	}

	if bytes.Compare(fw.Bytes(), bytes.Repeat(sampleData, 10)) != 0 {
		t.Errorf("copied bytes don't match!")
	}
}

func TestErrAlreadyStarted(t *testing.T) {
	cpp := NewCopyProgressPrinter()
	fr := &fakeReader{make(chan []byte, 1), false}
	fw := &bytes.Buffer{}

	out := &bytes.Buffer{}

	err := cpp.AddCopy(fr, "download", 10^10, fw)
	if err != nil {
		t.Errorf("%v\n", err)
	}

	cancel := make(chan struct{})
	doneChan := make(chan error)
	go func() {
		doneChan <- cpp.PrintAndWait(out, time.Second, cancel)
	}()

	for {
		cpp.lock.Lock()
		started := cpp.started
		cpp.lock.Unlock()
		if !started {
			time.Sleep(time.Millisecond * 5)
		} else {
			break
		}
	}

	err = cpp.AddCopy(fr, "download", 10^10, fw)
	if err != ErrAlreadyStarted {
		t.Errorf("Was expecting ErrAlreadyStarted, got something else: %v\n", err)
	}

	err = cpp.PrintAndWait(out, time.Second, cancel)
	if err != ErrAlreadyStarted {
		t.Errorf("Was expecting ErrAlreadyStarted, got something else: %v\n", err)
	}

	cancel <- struct{}{}

	err = <-doneChan
	if err != nil {
		t.Errorf("%v\n", err)
	}
}
