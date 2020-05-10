package goplay

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

type fakeTransPort struct {
	FakeRoundTrip func(*http.Request) (*http.Response, error)
}

func (f *fakeTransPort) RoundTrip(r *http.Request) (*http.Response, error) {
	return f.FakeRoundTrip(r)
}

func TestClient_Run(t *testing.T) {
	saveDelay := delay
	defer func() { delay = saveDelay }()

	events := []*Event{
		{Message: "out1", Kind: "stdout", Delay: 5},
		{Message: "out2", Kind: "stdout", Delay: 5},
		{Message: "err1", Kind: "stderr", Delay: 5},
	}

	delayCalledN := 0
	delay = func(d time.Duration) {
		delayCalledN++
		if d != 5 {
			t.Errorf("delay func got unexpected value: %v", d)
		}
	}
	defer func() {
		if got, want := delayCalledN, len(events); got != want {
			t.Errorf("delay func called %v times, want %v times", got, delayCalledN)
		}
	}()

	mu := http.NewServeMux()
	mu.HandleFunc("/compile", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method: got %v, want %v", r.Method, http.MethodPost)
		}
		if err := json.NewEncoder(w).Encode(&Response{Events: events}); err != nil {
			t.Error(err)
		}
	})
	ts := httptest.NewServer(mu)
	defer ts.Close()

	cli := &Client{
		BaseURL: ts.URL,
	}

	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)

	if err := cli.Run(bytes.NewReader([]byte("code")), stdout, stderr); err != nil {
		t.Fatal(err)
	}

	wantStdout := ""
	wantStderr := ""
	for _, e := range events {
		if e.Kind == "stdout" {
			wantStdout += e.Message
		} else {
			wantStderr += e.Message
		}
	}

	if got := stdout.String(); got != wantStdout {
		t.Errorf("Run() writes %v to stdout, want %v", got, wantStdout)
	}
	if got := stderr.String(); got != wantStderr {
		t.Errorf("Run() writes %v to stderr, want %v", got, wantStderr)
	}
}

var mockRunClient = &http.Client{Transport: &fakeTransPort{
	FakeRoundTrip: func(r *http.Request) (*http.Response, error) {
		mes := []*Event{
			{Message: "Hello, 世界!\n", Kind: "stdout", Delay: 0},
		}
		b, _ := json.Marshal(&Response{Errors: "", Events: mes})
		return &http.Response{Body: ioutil.NopCloser(bytes.NewReader(b))}, nil
	},
}}

func ExampleClient_Run() {
	code := `
package main

import "fmt"

func main() {
	fmt.Println("Hello, 世界!")
}
`
	// You can use `DefaultClient` instead of creating `Client` if you want
	cli := &Client{HTTPClient: mockRunClient}
	if err := cli.Run(strings.NewReader(code), os.Stdout, os.Stderr); err != nil {
		log.Fatal(err)
	}
	// Output: Hello, 世界!
}

func TestClient_Compile(t *testing.T) {
	wantResp := &Response{
		Errors: "",
		Events: []*Event{
			{Message: "test1", Kind: "stdout", Delay: 0},
			{Message: "test2", Kind: "stdout", Delay: 2},
		},
	}

	mu := http.NewServeMux()
	mu.HandleFunc("/compile", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method: got %v, want %v", r.Method, http.MethodPost)
		}
		if err := json.NewEncoder(w).Encode(wantResp); err != nil {
			t.Error(err)
		}
	})
	ts := httptest.NewServer(mu)
	defer ts.Close()

	cli := &Client{
		BaseURL: ts.URL,
	}

	got, err := cli.Compile(bytes.NewReader([]byte("code")))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, wantResp) {
		t.Errorf("Compile(code) == %v, want %v", got, wantResp)
	}
}

var mockCompileClient = &http.Client{Transport: &fakeTransPort{
	FakeRoundTrip: func(r *http.Request) (*http.Response, error) {
		mes := []*Event{
			{Message: "3...\n", Kind: "stdout", Delay: 0},
			{Message: "2...\n", Kind: "stdout", Delay: 1 * time.Second},
			{Message: "1...\n", Kind: "stdout", Delay: 1 * time.Second},
			{Message: "GO!\n", Kind: "stdout", Delay: 1 * time.Second},
		}
		b, _ := json.Marshal(&Response{Errors: "", Events: mes})
		return &http.Response{Body: ioutil.NopCloser(bytes.NewReader(b))}, nil
	},
}}

func ExampleClient_Compile() {
	code := `
package main

import (
	"fmt"
	"time"
)

func main() {
	for i := 3; i > 0; i-- {
		fmt.Printf("%d...\n", i)
		time.Sleep(1 * time.Second)
	}
	fmt.Println("GO!")
}
`
	// You can use `DefaultClient` instead of creating `Client` if you want
	cli := &Client{HTTPClient: mockCompileClient}
	resp, err := cli.Compile(strings.NewReader(code))
	if err != nil {
		log.Fatal(err)
	}
	for _, e := range resp.Events {
		// do not emulate delay.
		// time.Sleep(e.Delay)
		fmt.Print(e.Message)
	}
	// Output:
	// 3...
	// 2...
	// 1...
	// GO!
}

func TestClient_Share(t *testing.T) {
	mu := http.NewServeMux()
	mu.HandleFunc("/share", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method: got %v, want %v", r.Method, http.MethodPost)
		}
		fmt.Fprint(w, "xxx") // token
	})
	ts := httptest.NewServer(mu)
	defer ts.Close()

	cli := &Client{
		BaseURL: ts.URL,
	}

	link, err := cli.Share(bytes.NewReader([]byte("test")))
	if err != nil {
		t.Fatal(err)
	}
	if want := fmt.Sprintf("%s/p/xxx", ts.URL); link != want {
		t.Errorf("Share(code) == %v, want %v", link, want)
	}
}

var mockShareClient = &http.Client{Transport: &fakeTransPort{
	FakeRoundTrip: func(r *http.Request) (*http.Response, error) {
		return &http.Response{Body: ioutil.NopCloser(strings.NewReader("OclbDkg7kv"))}, nil
	},
}}

func ExampleClient_Share() {
	const code = `
package main

import "fmt"

func main() {
	fmt.Println("Hello, 世界!")
}
`
	// You can use `DefaultClient` instead of creating `Client` if you want
	cli := &Client{HTTPClient: mockShareClient}
	url, err := cli.Share(strings.NewReader(code))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(url)
	// Output: https://play.golang.org/p/OclbDkg7kv
}
