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

	"github.com/coreos/discovery.etcd.io/handlers/httperror"
	"github.com/prometheus/client_golang/prometheus"

	"go.etcd.io/etcd/client"
	"go.etcd.io/etcd/clientv3"
	"go.etcd.io/etcd/etcdserver/api/v2store"
	"go.etcd.io/etcd/etcdserver/api/v2v3"
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

func (st *State) setupToken(size int) (string, error) {
	token := generateCluster()
	if token == "" {
		return "", errors.New("Couldn't generate a token")
	}

	println("philips create")

	ev, err := st.v2.Create(path.Join(token, "config", "size"), false, strconv.Itoa(size), false, v2store.TTLOptionSet{})
	if err != nil {
		return "", fmt.Errorf("Couldn't setup state %v %v", ev, err)
	}

	println("philips create done")

	return token, nil
}

func (st *State) deleteToken(token string) error {
	c, _ := client.New(client.Config{
		Endpoints: []string{st.endpoint()},
		Transport: client.DefaultTransport,
		// set timeout per request to fail fast when the target endpoint is unavailable
		HeaderTimeoutPerRequest: time.Second,
	})
	kapi := client.NewKeysAPI(c)

	if token == "" {
		return errors.New("No token given")
	}

	_, err := kapi.Delete(
		context.Background(),
		path.Join("_etcd", "registry", token),
		&client.DeleteOptions{Recursive: true},
	)
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
	token, err := st.setupToken(size)

	if err != nil {
		log.Printf("setupToken returned: %v", err)
		httperror.Error(w, r, "Unable to generate token", 400, newCounter)
		return
	}

	log.Println("New cluster created", token)

	fmt.Fprintf(w, "%s/%s", bytes.TrimRight([]byte(st.discHost), "/"), token)
	newCounter.WithLabelValues("200", r.Method).Add(1)
}
