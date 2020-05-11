package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/golang/protobuf/jsonpb"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	spb "google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc/codes"
)

func TestWithProtoErrorHandler(t *testing.T) {
	go func() {
		if err := Run(
			":8082",
			runtime.WithProtoErrorHandler(runtime.DefaultHTTPProtoErrorHandler),
		); err != nil {
			t.Errorf("gw.Run() failed with %v; want success", err)
			return
		}
	}()

	time.Sleep(100 * time.Millisecond)
	testEcho(t, 8082, "application/json")
	testEchoBody(t, 8082)
}

func TestABEWithProtoErrorHandler(t *testing.T) {
	if testing.Short() {
		t.Skip()
		return
	}

	testABECreate(t, 8082)
	testABECreateBody(t, 8082)
	testABEBulkCreate(t, 8082)
	testABELookup(t, 8082)
	testABELookupNotFoundWithProtoError(t)
	testABEList(t, 8082)
	testABEBulkEcho(t, 8082)
	testABEBulkEchoZeroLength(t, 8082)
	testAdditionalBindings(t, 8082)
}

func testABELookupNotFoundWithProtoError(t *testing.T) {
	url := "http://localhost:8082/v1/example/a_bit_of_everything"
	uuid := "not_exist"
	url = fmt.Sprintf("%s/%s", url, uuid)
	resp, err := http.Get(url)
	if err != nil {
		t.Errorf("http.Get(%q) failed with %v; want success", url, err)
		return
	}
	defer resp.Body.Close()

	buf, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Errorf("ioutil.ReadAll(resp.Body) failed with %v; want success", err)
		return
	}

	if got, want := resp.StatusCode, http.StatusNotFound; got != want {
		t.Errorf("resp.StatusCode = %d; want %d", got, want)
		t.Logf("%s", buf)
		return
	}

	var msg spb.Status
	if err := jsonpb.UnmarshalString(string(buf), &msg); err != nil {
		t.Errorf("jsonpb.UnmarshalString(%s, &msg) failed with %v; want success", buf, err)
		return
	}

	if got, want := msg.Code, int32(codes.NotFound); got != want {
		t.Errorf("msg.Code = %d; want %d", got, want)
		return
	}

	if got, want := msg.Message, "not found"; got != want {
		t.Errorf("msg.Message = %s; want %s", got, want)
		return
	}

	if got, want := resp.Header.Get("Grpc-Metadata-Uuid"), uuid; got != want {
		t.Errorf("Grpc-Metadata-Uuid was %s, wanted %s", got, want)
	}
	if got, want := resp.Trailer.Get("Grpc-Trailer-Foo"), "foo2"; got != want {
		t.Errorf("Grpc-Trailer-Foo was %q, wanted %q", got, want)
	}
	if got, want := resp.Trailer.Get("Grpc-Trailer-Bar"), "bar2"; got != want {
		t.Errorf("Grpc-Trailer-Bar was %q, wanted %q", got, want)
	}
}

func TestUnknownPathWithProtoError(t *testing.T) {
	url := "http://localhost:8082"
	resp, err := http.Post(url, "application/json", strings.NewReader("{}"))
	if err != nil {
		t.Errorf("http.Post(%q) failed with %v; want success", url, err)
		return
	}
	defer resp.Body.Close()
	buf, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Errorf("iotuil.ReadAll(resp.Body) failed with %v; want success", err)
		return
	}

	if got, want := resp.StatusCode, http.StatusNotImplemented; got != want {
		t.Errorf("resp.StatusCode = %d; want %d", got, want)
		t.Logf("%s", buf)
	}

	var msg spb.Status
	if err := jsonpb.UnmarshalString(string(buf), &msg); err != nil {
		t.Errorf("jsonpb.UnmarshalString(%s, &msg) failed with %v; want success", buf, err)
		return
	}

	if got, want := msg.Code, int32(codes.Unimplemented); got != want {
		t.Errorf("msg.Code = %d; want %d", got, want)
		return
	}

	if msg.Message == "" {
		t.Errorf("msg.Message should not be empty")
		return
	}
}

func TestMethodNotAllowedWithProtoError(t *testing.T) {
	url := "http://localhost:8082/v1/example/echo/myid"
	resp, err := http.Get(url)
	if err != nil {
		t.Errorf("http.Post(%q) failed with %v; want success", url, err)
		return
	}
	defer resp.Body.Close()
	buf, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Errorf("iotuil.ReadAll(resp.Body) failed with %v; want success", err)
		return
	}

	if got, want := resp.StatusCode, http.StatusNotImplemented; got != want {
		t.Errorf("resp.StatusCode = %d; want %d", got, want)
		t.Logf("%s", buf)
	}

	var msg spb.Status
	if err := jsonpb.UnmarshalString(string(buf), &msg); err != nil {
		t.Errorf("jsonpb.UnmarshalString(%s, &msg) failed with %v; want success", buf, err)
		return
	}

	if got, want := msg.Code, int32(codes.Unimplemented); got != want {
		t.Errorf("msg.Code = %d; want %d", got, want)
		return
	}

	if msg.Message == "" {
		t.Errorf("msg.Message should not be empty")
		return
	}
}
