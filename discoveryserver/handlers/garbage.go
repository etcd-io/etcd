package handlers

import (
	"context"
	"log"
	"path"
	"time"

	"go.etcd.io/etcd/clientv3"
	"go.etcd.io/etcd/clientv3/concurrency"

	"go.etcd.io/etcd/discoveryserver/timeprefix"
)

func discoveryGCPath() string {
	return path.Join("/", "discoverygc")
}

func (st *State) lockGC() *concurrency.Mutex {
	if st.session == nil {
		s, err := concurrency.NewSession(st.client)
		if err != nil {
			log.Fatal(err)
		}
		st.session = s
	}
	m := concurrency.NewMutex(st.session, discoveryGCPath()+"lock")

	if err := m.Lock(context.Background()); err != nil {
		log.Printf("GC lock error: %v", err)
		return nil
	}

	return m
}

func (st *State) GarbageCollect(min time.Duration, max time.Duration) {
	m := st.lockGC()
	if m == nil {
		log.Printf("no lock, not GC'ing")
	}
	p := timeprefix.Prefixes(min, max)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	resp, err := st.client.Get(ctx, path.Join(discoveryGCPath(), p[0]), clientv3.WithRange(path.Join(discoveryGCPath(), p[1])))
	cancel()

	if err != nil {
		log.Printf("couldn't get range for GC %v", err)
	}

	for _, k := range resp.Kvs {
		log.Printf("gc key %v", string(k.Key))
		b := path.Base(string(k.Key))
		err := st.deleteToken(b, string(k.Key))
		if err != nil {
			log.Printf("couldn't delete token for GC: %v", err)
			continue
		}
	}

	if err := m.Unlock(context.TODO()); err != nil {
		log.Fatal(err)
	}
}
