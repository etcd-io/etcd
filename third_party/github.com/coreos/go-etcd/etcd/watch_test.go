package etcd

import (
	"fmt"
	"testing"
	"time"
)

func TestWatch(t *testing.T) {
	c := NewClient(nil)
	defer func() {
		c.DeleteAll("watch_foo")
	}()

	go setHelper("watch_foo", "bar", c)

	resp, err := c.Watch("watch_foo", 0, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !(resp.Key == "/watch_foo" && resp.Value == "bar") {
		t.Fatalf("Watch 1 failed: %#v", resp)
	}

	go setHelper("watch_foo", "bar", c)

	resp, err = c.Watch("watch_foo", resp.ModifiedIndex, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !(resp.Key == "/watch_foo" && resp.Value == "bar") {
		t.Fatalf("Watch 2 failed: %#v", resp)
	}

	ch := make(chan *Response, 10)
	stop := make(chan bool, 1)

	go setLoop("watch_foo", "bar", c)

	go receiver(ch, stop)

	_, err = c.Watch("watch_foo", 0, ch, stop)
	if err != ErrWatchStoppedByUser {
		t.Fatalf("Watch returned a non-user stop error")
	}
}

func TestWatchAll(t *testing.T) {
	c := NewClient(nil)
	defer func() {
		c.DeleteAll("watch_foo")
	}()

	go setHelper("watch_foo/foo", "bar", c)

	resp, err := c.WatchAll("watch_foo", 0, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !(resp.Key == "/watch_foo/foo" && resp.Value == "bar") {
		t.Fatalf("WatchAll 1 failed: %#v", resp)
	}

	go setHelper("watch_foo/foo", "bar", c)

	resp, err = c.WatchAll("watch_foo", resp.ModifiedIndex, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !(resp.Key == "/watch_foo/foo" && resp.Value == "bar") {
		t.Fatalf("WatchAll 2 failed: %#v", resp)
	}

	ch := make(chan *Response, 10)
	stop := make(chan bool, 1)

	go setLoop("watch_foo/foo", "bar", c)

	go receiver(ch, stop)

	_, err = c.WatchAll("watch_foo", 0, ch, stop)
	if err != ErrWatchStoppedByUser {
		t.Fatalf("Watch returned a non-user stop error")
	}
}

func setHelper(key, value string, c *Client) {
	time.Sleep(time.Second)
	c.Set(key, value, 100)
}

func setLoop(key, value string, c *Client) {
	time.Sleep(time.Second)
	for i := 0; i < 10; i++ {
		newValue := fmt.Sprintf("%s_%v", value, i)
		c.Set(key, newValue, 100)
		time.Sleep(time.Second / 10)
	}
}

func receiver(c chan *Response, stop chan bool) {
	for i := 0; i < 10; i++ {
		<-c
	}
	stop <- true
}
