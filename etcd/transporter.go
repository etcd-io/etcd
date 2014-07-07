package etcd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"path"
	"sync"

	"github.com/coreos/etcd/raft"
	"github.com/coreos/etcd/store"
)

var (
	errUnknownNode = errors.New("unknown node")
)

type transporter struct {
	mu      sync.RWMutex
	stopped bool
	urls    map[int]string

	recv   chan *raft.Message
	client *http.Client
	wg     sync.WaitGroup
}

func newTransporter() *transporter {
	tr := new(http.Transport)
	c := &http.Client{Transport: tr}

	return &transporter{
		urls:   make(map[int]string),
		recv:   make(chan *raft.Message, 512),
		client: c,
	}
}

func (t *transporter) stop() {
	t.mu.Lock()
	t.stopped = true
	t.mu.Unlock()

	t.wg.Wait()
	tr := t.client.Transport.(*http.Transport)
	tr.CloseIdleConnections()
}

func (t *transporter) set(nodeId int, rawurl string) error {
	u, err := url.Parse(rawurl)
	if err != nil {
		return err
	}
	u.Path = raftPrefix
	t.mu.Lock()
	t.urls[nodeId] = u.String()
	t.mu.Unlock()
	return nil
}

func (t *transporter) sendTo(nodeId int, data []byte) error {
	t.mu.RLock()
	url := t.urls[nodeId]
	t.mu.RUnlock()

	if len(url) == 0 {
		return errUnknownNode
	}
	return t.send(url, data)
}

func (t *transporter) send(addr string, data []byte) error {
	t.mu.RLock()
	if t.stopped {
		t.mu.RUnlock()
		return fmt.Errorf("transporter stopped")
	}
	t.mu.RUnlock()

	buf := bytes.NewBuffer(data)
	t.wg.Add(1)
	defer t.wg.Done()
	resp, err := t.client.Post(addr, "application/octet-stream", buf)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func (t *transporter) fetchAddr(seedurl string, id int) error {
	u, err := url.Parse(seedurl)
	if err != nil {
		return fmt.Errorf("cannot parse the url of the given seed")
	}

	u.Path = path.Join(v2Prefix, v2machineKVPrefix, fmt.Sprint(id))
	resp, err := t.client.Get(u.String())
	if err != nil {
		return fmt.Errorf("cannot reach %v", u)
	}
	defer resp.Body.Close()

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("cannot reach %v", u)
	}

	event := new(store.Event)
	err = json.Unmarshal(b, event)
	if err != nil {
		panic(fmt.Sprintf("fetchAddr: ", err))
	}

	if err := t.set(id, *event.Node.Value); err != nil {
		return fmt.Errorf("cannot parse the url of node %d: %v", id, err)
	}
	return nil
}

func (t *transporter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	msg := new(raft.Message)
	if err := json.NewDecoder(r.Body).Decode(msg); err != nil {
		log.Println(err)
		return
	}

	select {
	case t.recv <- msg:
	default:
		log.Println("drop")
		// drop the incoming package at network layer if the upper layer
		// cannot consume them in time.
		// TODO(xiangli): not return 200.
	}
	return
}
