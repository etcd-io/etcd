package main

import (
	"crypto/tls"
	"testing"
	"time"
)

func TestTransporterTimeout(t *testing.T) {

	conf := tls.Config{}

	ts := newTransporter("http", conf, time.Second)

	ts.Get("http://google.com")
	_, err := ts.Get("http://google.com:9999") // it doesn't exisit
	if err == nil || err.Error() != "Wait Response Timeout: 1s" {
		t.Fatal("timeout error: ", err.Error())
	}

	_, err = ts.Post("http://google.com:9999", nil) // it doesn't exisit
	if err == nil || err.Error() != "Wait Response Timeout: 1s" {
		t.Fatal("timeout error: ", err.Error())
	}

	_, err = ts.Get("http://www.google.com")
	if err != nil {
		t.Fatal("get error")
	}

	_, err = ts.Post("http://www.google.com", nil)
	if err != nil {
		t.Fatal("post error")
	}

}
