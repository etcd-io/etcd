package handlers

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"time"

	"go.etcd.io/etcd/clientv3"
	"go.etcd.io/etcd/discoveryserver/handlers/httperror"
	"go.etcd.io/etcd/discoveryserver/timeprefix"
	"go.etcd.io/etcd/etcdserver/api/v2store"
	"go.etcd.io/etcd/etcdserver/api/v2v3"

	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc/grpclog"
)

var newCounter *prometheus.CounterVec

func init() {
	newCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "endpoint_new_requests_total",
			Help: "How many /new requests processed, partitioned by status code and HTTP method.",
		},
		[]string{"code", "method"},
	)
	prometheus.MustRegister(newCounter)
}

func generateCluster() string {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		return ""
	}

	return hex.EncodeToString(b)
}

func Setup(etcdCURL, disc string) *State {
	u, _ := url.Parse(etcdCURL)

	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{etcdCURL},
		DialTimeout: 5 * time.Second,
	})
	clientv3.SetLogger(grpclog.NewLoggerV2(os.Stdout, os.Stdout, os.Stdout))
	if err != nil {
		panic(err)
	}

	prefix := path.Join("/", "discovery.etcd.io", "registry")
	v2 := v2v3.NewStore(cli, prefix)

	return &State{
		etcdHost:      etcdCURL,
		etcdCURL:      u,
		currentLeader: u.Host,
		discHost:      disc,
		client:        cli,
		v2:            v2,
	}
}

func (st *State) setupToken(size int) (string, string, error) {
	token := generateCluster()
	if token == "" {
		return "", "", errors.New("couldn't generate a token")
	}

	ev, err := st.v2.Create(path.Join("/", token, "_config", "size"), false, strconv.Itoa(size), false, v2store.TTLOptionSet{})
	if err != nil {
		return "", "", fmt.Errorf("couldn't setup state %v %v", ev, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	gckey := path.Join(discoveryGCPath(), timeprefix.Now(), token)
	_, err = st.client.Put(ctx, gckey, "")
	cancel()
	if err != nil {
		return "", "", fmt.Errorf("couldn't setup state %v %v", ev, err)
	}

	return token, gckey, nil
}

func (st *State) deleteToken(token string, gckey string) error {
	// TODO(philips): potential orphan index if v2.Delete
	// succeeds but gckey deletion doesn't.
	_, err := st.v2.Delete(path.Join("/", token), true, true)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	_, err = st.client.Delete(ctx, gckey)
	cancel()
	return err
}

func NewTokenHandler(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	st := ctx.Value(stateKey).(*State)

	var err error
	size := 3
	s := r.FormValue("size")
	if s != "" {
		size, err = strconv.Atoi(s)
		if err != nil {
			httperror.Error(w, r, err.Error(), http.StatusBadRequest, newCounter)
			return
		}
	}
	token, _, err := st.setupToken(size)

	if err != nil {
		log.Printf("setupToken returned: %v", err)
		httperror.Error(w, r, "unable to generate token", 400, newCounter)
		return
	}

	log.Println("New cluster created", token)

	fmt.Fprintf(w, "%s/%s", bytes.TrimRight([]byte(st.discHost), "/"), token)
	newCounter.WithLabelValues("200", r.Method).Add(1)
}
