package server

/*
import (
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net/http"
	"testing"
	"time"
)

func TestTransporterTimeout(t *testing.T) {

	http.HandleFunc("/timeout", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "timeout")
		w.(http.Flusher).Flush() // send headers and some body
		time.Sleep(time.Second * 100)
	})

	go http.ListenAndServe(":8080", nil)

	conf := tls.Config{}

	ts := newTransporter("http", conf, nil)

	ts.Get("http://google.com")
	_, _, err := ts.Get("http://google.com:9999")
	if err == nil {
		t.Fatal("timeout error")
	}

	res, req, err := ts.Get("http://localhost:8080/timeout")

	if err != nil {
		t.Fatal("should not timeout")
	}

	ts.CancelWhenTimeout(req)

	body, err := ioutil.ReadAll(res.Body)
	if err == nil {
		fmt.Println(string(body))
		t.Fatal("expected an error reading the body")
	}

	_, _, err = ts.Post("http://google.com:9999", nil)
	if err == nil {
		t.Fatal("timeout error")
	}

	_, _, err = ts.Get("http://www.google.com")
	if err != nil {
		t.Fatal("get error: ", err.Error())
	}

	_, _, err = ts.Post("http://www.google.com", nil)
	if err != nil {
		t.Fatal("post error")
	}

}
*/
