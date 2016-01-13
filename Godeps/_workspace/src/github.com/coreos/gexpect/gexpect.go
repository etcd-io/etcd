package gexpect

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"time"
	"unicode/utf8"

	shell "github.com/coreos/etcd/Godeps/_workspace/src/github.com/kballard/go-shellquote"
	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/kr/pty"
)

type ExpectSubprocess struct {
	Cmd          *exec.Cmd
	buf          *buffer
	outputBuffer []byte
}

type buffer struct {
	f       *os.File
	b       bytes.Buffer
	collect bool

	collection bytes.Buffer
}

func (buf *buffer) StartCollecting() {
	buf.collect = true
}

func (buf *buffer) StopCollecting() (result string) {
	result = string(buf.collection.Bytes())
	buf.collect = false
	buf.collection.Reset()
	return result
}

func (buf *buffer) Read(chunk []byte) (int, error) {
	nread := 0
	if buf.b.Len() > 0 {
		n, err := buf.b.Read(chunk)
		if err != nil {
			return n, err
		}
		if n == len(chunk) {
			return n, nil
		}
		nread = n
	}
	fn, err := buf.f.Read(chunk[nread:])
	return fn + nread, err
}

func (buf *buffer) ReadRune() (r rune, size int, err error) {
	l := buf.b.Len()

	chunk := make([]byte, utf8.UTFMax)
	if l > 0 {
		n, err := buf.b.Read(chunk)
		if err != nil {
			return 0, 0, err
		}
		if utf8.FullRune(chunk) {
			r, rL := utf8.DecodeRune(chunk)
			if n > rL {
				buf.PutBack(chunk[rL:n])
			}
			if buf.collect {
				buf.collection.WriteRune(r)
			}
			return r, rL, nil
		}
	}
	// else add bytes from the file, then try that
	for l < utf8.UTFMax {
		fn, err := buf.f.Read(chunk[l : l+1])
		if err != nil {
			return 0, 0, err
		}
		l = l + fn

		if utf8.FullRune(chunk) {
			r, rL := utf8.DecodeRune(chunk)
			if buf.collect {
				buf.collection.WriteRune(r)
			}
			return r, rL, nil
		}
	}
	return 0, 0, errors.New("File is not a valid UTF=8 encoding")
}

func (buf *buffer) PutBack(chunk []byte) {
	if len(chunk) == 0 {
		return
	}
	if buf.b.Len() == 0 {
		buf.b.Write(chunk)
		return
	}
	d := make([]byte, 0, len(chunk)+buf.b.Len())
	d = append(d, chunk...)
	d = append(d, buf.b.Bytes()...)
	buf.b.Reset()
	buf.b.Write(d)
}

func SpawnAtDirectory(command string, directory string) (*ExpectSubprocess, error) {
	expect, err := _spawn(command)
	if err != nil {
		return nil, err
	}
	expect.Cmd.Dir = directory
	return _start(expect)
}

func Command(command string) (*ExpectSubprocess, error) {
	expect, err := _spawn(command)
	if err != nil {
		return nil, err
	}
	return expect, nil
}

func (expect *ExpectSubprocess) Start() error {
	_, err := _start(expect)
	return err
}

func Spawn(command string) (*ExpectSubprocess, error) {
	expect, err := _spawn(command)
	if err != nil {
		return nil, err
	}
	return _start(expect)
}

func (expect *ExpectSubprocess) Close() error {
	return expect.Cmd.Process.Kill()
}

func (expect *ExpectSubprocess) AsyncInteractChannels() (send chan string, receive chan string) {
	receive = make(chan string)
	send = make(chan string)

	go func() {
		for {
			str, err := expect.ReadLine()
			if err != nil {
				close(receive)
				return
			}
			receive <- str
		}
	}()

	go func() {
		for {
			select {
			case sendCommand, exists := <-send:
				{
					if !exists {
						return
					}
					err := expect.Send(sendCommand)
					if err != nil {
						receive <- "gexpect Error: " + err.Error()
						return
					}
				}
			}
		}
	}()

	return
}

func (expect *ExpectSubprocess) ExpectRegex(regex string) (bool, error) {
	return regexp.MatchReader(regex, expect.buf)
}

func (expect *ExpectSubprocess) expectRegexFind(regex string, output bool) ([]string, string, error) {
	re, err := regexp.Compile(regex)
	if err != nil {
		return nil, "", err
	}
	expect.buf.StartCollecting()
	pairs := re.FindReaderSubmatchIndex(expect.buf)
	stringIndexedInto := expect.buf.StopCollecting()
	l := len(pairs)
	numPairs := l / 2
	result := make([]string, numPairs)
	for i := 0; i < numPairs; i += 1 {
		result[i] = stringIndexedInto[pairs[i*2]:pairs[i*2+1]]
	}
	// convert indexes to strings

	if len(result) == 0 {
		err = fmt.Errorf("ExpectRegex didn't find regex '%v'.", regex)
	}
	return result, stringIndexedInto, err
}

func (expect *ExpectSubprocess) expectTimeoutRegexFind(regex string, timeout time.Duration) (result []string, out string, err error) {
	t := make(chan bool)
	go func() {
		result, out, err = expect.ExpectRegexFindWithOutput(regex)
		t <- false
	}()
	go func() {
		time.Sleep(timeout)
		err = fmt.Errorf("ExpectRegex timed out after %v finding '%v'.\nOutput:\n%s", timeout, regex, expect.Collect())
		t <- true
	}()
	<-t
	return result, out, err
}

func (expect *ExpectSubprocess) ExpectRegexFind(regex string) ([]string, error) {
	result, _, err := expect.expectRegexFind(regex, false)
	return result, err
}

func (expect *ExpectSubprocess) ExpectTimeoutRegexFind(regex string, timeout time.Duration) ([]string, error) {
	result, _, err := expect.expectTimeoutRegexFind(regex, timeout)
	return result, err
}

func (expect *ExpectSubprocess) ExpectRegexFindWithOutput(regex string) ([]string, string, error) {
	return expect.expectRegexFind(regex, true)
}

func (expect *ExpectSubprocess) ExpectTimeoutRegexFindWithOutput(regex string, timeout time.Duration) ([]string, string, error) {
	return expect.expectTimeoutRegexFind(regex, timeout)
}

func buildKMPTable(searchString string) []int {
	pos := 2
	cnd := 0
	length := len(searchString)

	var table []int
	if length < 2 {
		length = 2
	}

	table = make([]int, length)
	table[0] = -1
	table[1] = 0

	for pos < len(searchString) {
		if searchString[pos-1] == searchString[cnd] {
			cnd += 1
			table[pos] = cnd
			pos += 1
		} else if cnd > 0 {
			cnd = table[cnd]
		} else {
			table[pos] = 0
			pos += 1
		}
	}
	return table
}

func (expect *ExpectSubprocess) ExpectTimeout(searchString string, timeout time.Duration) (e error) {
	result := make(chan error)
	go func() {
		result <- expect.Expect(searchString)
	}()
	select {
	case e = <-result:
	case <-time.After(timeout):
		e = fmt.Errorf("Expect timed out after %v waiting for '%v'.\nOutput:\n%s", timeout, searchString, expect.Collect())
	}
	return e
}

func (expect *ExpectSubprocess) Expect(searchString string) (e error) {
	chunk := make([]byte, len(searchString)*2)
	target := len(searchString)
	if expect.outputBuffer != nil {
		expect.outputBuffer = expect.outputBuffer[0:]
	}
	m := 0
	i := 0
	// Build KMP Table
	table := buildKMPTable(searchString)

	for {
		n, err := expect.buf.Read(chunk)

		if err != nil {
			return err
		}
		if expect.outputBuffer != nil {
			expect.outputBuffer = append(expect.outputBuffer, chunk[:n]...)
		}
		offset := m + i
		for m+i-offset < n {
			if searchString[i] == chunk[m+i-offset] {
				i += 1
				if i == target {
					unreadIndex := m + i - offset
					if len(chunk) > unreadIndex {
						expect.buf.PutBack(chunk[unreadIndex:])
					}
					return nil
				}
			} else {
				m += i - table[i]
				if table[i] > -1 {
					i = table[i]
				} else {
					i = 0
				}
			}
		}
	}
}

func (expect *ExpectSubprocess) Send(command string) error {
	_, err := io.WriteString(expect.buf.f, command)
	return err
}

func (expect *ExpectSubprocess) Capture() {
	if expect.outputBuffer == nil {
		expect.outputBuffer = make([]byte, 0)
	}
}

func (expect *ExpectSubprocess) Collect() []byte {
	collectOutput := make([]byte, len(expect.outputBuffer))
	copy(collectOutput, expect.outputBuffer)
	expect.outputBuffer = nil
	return collectOutput
}

func (expect *ExpectSubprocess) SendLine(command string) error {
	_, err := io.WriteString(expect.buf.f, command+"\r\n")
	return err
}

func (expect *ExpectSubprocess) Interact() {
	defer expect.Cmd.Wait()
	io.Copy(os.Stdout, &expect.buf.b)
	go io.Copy(os.Stdout, expect.buf.f)
	go io.Copy(expect.buf.f, os.Stdin)
}

func (expect *ExpectSubprocess) ReadUntil(delim byte) ([]byte, error) {
	join := make([]byte, 1, 512)
	chunk := make([]byte, 255)

	for {

		n, err := expect.buf.Read(chunk)

		if err != nil {
			return join, err
		}

		for i := 0; i < n; i++ {
			if chunk[i] == delim {
				if len(chunk) > i+1 {
					expect.buf.PutBack(chunk[i+1:])
				}
				return join, nil
			} else {
				join = append(join, chunk[i])
			}
		}
	}
}

func (expect *ExpectSubprocess) Wait() error {
	return expect.Cmd.Wait()
}

func (expect *ExpectSubprocess) ReadLine() (string, error) {
	str, err := expect.ReadUntil('\n')
	if err != nil {
		return "", err
	}
	return string(str), nil
}

func _start(expect *ExpectSubprocess) (*ExpectSubprocess, error) {
	f, err := pty.Start(expect.Cmd)
	if err != nil {
		return nil, err
	}
	expect.buf.f = f

	return expect, nil
}

func _spawn(command string) (*ExpectSubprocess, error) {
	wrapper := new(ExpectSubprocess)

	wrapper.outputBuffer = nil

	splitArgs, err := shell.Split(command)
	if err != nil {
		return nil, err
	}
	numArguments := len(splitArgs) - 1
	if numArguments < 0 {
		return nil, errors.New("gexpect: No command given to spawn")
	}
	path, err := exec.LookPath(splitArgs[0])
	if err != nil {
		return nil, err
	}

	if numArguments >= 1 {
		wrapper.Cmd = exec.Command(path, splitArgs[1:]...)
	} else {
		wrapper.Cmd = exec.Command(path)
	}
	wrapper.buf = new(buffer)

	return wrapper, nil
}
