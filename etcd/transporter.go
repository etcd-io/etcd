package etcd

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"sync"

	"github.com/coreos/etcd/raft"
)

const (
	serving int = iota
	stopped
)

var (
	errUnknownNode = errors.New("unknown node")
)

type transporter struct {
	mu     sync.RWMutex
	status int
	urls   map[int64]string

	recv   chan *raft.Message
	client *http.Client
	wg     sync.WaitGroup
	*http.ServeMux
}

func newTransporter(tc *tls.Config) *transporter {
	tr := new(http.Transport)
	tr.TLSClientConfig = tc
	c := &http.Client{Transport: tr}

	t := &transporter{
		urls:   make(map[int64]string),
		recv:   make(chan *raft.Message, 512),
		client: c,
	}
	t.ServeMux = http.NewServeMux()
	t.ServeMux.HandleFunc("/raft/cfg/", t.serveCfg)
	t.ServeMux.HandleFunc("/raft", t.serveRaft)
	return t
}

func (t *transporter) start() {
	t.mu.Lock()
	t.status = serving
	t.mu.Unlock()
}

func (t *transporter) stop() {
	t.mu.Lock()
	t.status = stopped
	t.mu.Unlock()
}

func (t *transporter) closeConnections() {
	t.wg.Wait()
	tr := t.client.Transport.(*http.Transport)
	tr.CloseIdleConnections()
}

func (t *transporter) set(nodeId int64, rawurl string) error {
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

func (t *transporter) sendTo(nodeId int64, data []byte) error {
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
	if t.status == stopped {
		t.mu.RUnlock()
		return fmt.Errorf("transporter stopped")
	}
	t.wg.Add(1)
	defer t.wg.Done()
	t.mu.RUnlock()

	buf := bytes.NewBuffer(data)
	resp, err := t.client.Post(addr, "application/octet-stream", buf)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func (t *transporter) fetchAddr(seedurl string, id int64) error {
	u, err := url.Parse(seedurl)
	if err != nil {
		return fmt.Errorf("cannot parse the url of the given seed")
	}

	u.Path = path.Join("/raft/cfg", fmt.Sprint(id))
	resp, err := t.client.Get(u.String())
	if err != nil {
		return fmt.Errorf("cannot reach %v", u)
	}
	defer resp.Body.Close()

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("cannot reach %v", u)
	}

	if err := t.set(id, string(b)); err != nil {
		return fmt.Errorf("cannot parse the url of node %d: %v", id, err)
	}
	return nil
}

func (t *transporter) serveRaft(w http.ResponseWriter, r *http.Request) {
	t.mu.RLock()
	status := t.status
	t.mu.RUnlock()
	if status == stopped {
		http.Error(w, "404 page not found", http.StatusNotFound)
		return
	}

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

func (t *transporter) serveCfg(w http.ResponseWriter, r *http.Request) {
	t.mu.RLock()
	status := t.status
	t.mu.RUnlock()
	if status == stopped {
		http.Error(w, "404 page not found", http.StatusNotFound)
		return
	}

	id, err := strconv.ParseInt(r.URL.Path[len("/raft/cfg/"):], 10, 64)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	t.mu.RLock()
	u, ok := t.urls[id]
	t.mu.RUnlock()
	if ok {
		w.Write([]byte(u))
		return
	}
	http.Error(w, "Not Found", http.StatusNotFound)
}
